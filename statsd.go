/*
Package statsd is a StatsD-compatible client for collecting operational
in-app metrics.

Supports counting, sampling, timing, gauges, sets and multi-metrics packet.

Example usage:

	err := statsd.SetAddr("127.0.0.1:8125")
	if err != nil {
		// handle error
	}
	err = statsd.Increment("buckets", 1, 1)

*/
package statsd

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	defaultBufSize = 512
)

// Client is StatsD client that collects and sends in-app metrics.
type Client interface {

	// Increment increments the counter for the given bucket.
	Increment(stat string, count int, rate float64) error

	// Decrement decrements the counter for the given bucket.
	Decrement(stat string, count int, rate float64) error

	// Duration records time spent for the given bucket with time.Duration.
	Duration(stat string, duration time.Duration, rate float64) error

	// Timing records time spent for the given bucket in milliseconds.
	Timing(stat string, delta int, rate float64) error

	// Time calculates time spent in given function and send it.
	Time(stat string, rate float64, f func()) error

	// Gauge records arbitrary values for the given bucket.
	Gauge(stat string, value int, rate float64) error

	// IncrementGauge increments the value of the gauge.
	IncrementGauge(stat string, value int, rate float64) error

	// DecrementGauge decrements the value of the gauge.
	DecrementGauge(stat string, value int, rate float64) error

	// Unique records unique occurences of events.
	Unique(stat string, value int, rate float64) error

	// Flush flushes writes any buffered data to the network.
	Flush() error

	// Close closes the connection.
	Close() error
}

type client struct {
	addr string
	size int

	m    sync.Mutex
	buf  bytes.Buffer
	conn net.Conn
}

func millisecond(d time.Duration) int {
	return int(d.Seconds() * 1000)
}

func newClient(addr string) (*client, error) {
	c := &client{
		addr: addr,
		size: defaultBufSize,
	}
	return c, c.connect()
}

func (c *client) connect() error {
	conn, err := net.Dial("udp", c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Increment implements the Client interface.
func (c *client) Increment(stat string, count int, rate float64) error {
	return c.send(stat, rate, "%d|c", count)
}

// Decrement implements the Client interface.
func (c *client) Decrement(stat string, count int, rate float64) error {
	return c.Increment(stat, -count, rate)
}

// Duration implements the Client interface.
func (c *client) Duration(stat string, duration time.Duration, rate float64) error {
	return c.send(stat, rate, "%d|ms", millisecond(duration))
}

// Timing implements the Client interface.
func (c *client) Timing(stat string, delta int, rate float64) error {
	return c.send(stat, rate, "%d|ms", delta)
}

// Time implements the Client interface.
func (c *client) Time(stat string, rate float64, f func()) error {
	ts := time.Now()
	f()
	return c.Duration(stat, time.Since(ts), rate)
}

// Gauge implements the Client interface.
func (c *client) Gauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "%d|g", value)
}

// IncrementGauge implements the Client interface.
func (c *client) IncrementGauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "+%d|g", value)
}

// DecrementGauge implements the Client interface.
func (c *client) DecrementGauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "-%d|g", value)
}

// Unique implements the Client interface.
func (c *client) Unique(stat string, value int, rate float64) error {
	return c.send(stat, rate, "%d|s", value)
}

// Flush implements the Client interface.
func (c *client) Flush() error {
	defer c.buf.Reset()

	if c.conn == nil {
		err := c.connect()
		if err != nil {
			return err
		}
	}

	_, err := c.conn.Write(c.buf.Bytes())
	if err != nil {
		// Try to reconnect and retry
		err = c.connect()
		if err != nil {
			return err
		}
		_, err = c.conn.Write(c.buf.Bytes())
		return err
	}

	return nil
}

// Close implements the Client interface.
func (c *client) Close() error {
	if err := c.Flush(); err != nil {
		return err
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *client) send(stat string, rate float64, format string, args ...interface{}) error {
	if rate < 1 {
		if rand.Float64() < rate {
			format = fmt.Sprintf("%s|@%g", format, rate)
		} else {
			return nil
		}
	}

	format = fmt.Sprintf("%s:%s", stat, format)

	c.m.Lock()
	defer c.m.Unlock()

	var err error

	// Flush data if we have reach the buffer limit
	if (c.size - c.buf.Len()) < len(format) {
		err = c.Flush()
		if err != nil {
			return err
		}
	}

	// Buffer is not empty, start filling it
	if c.buf.Len() > 0 {
		_, err = fmt.Fprint(&c.buf, "\n")
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(&c.buf, format, args...)
	if err != nil {
		return err
	}

	return nil
}
