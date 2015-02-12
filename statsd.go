/*
Package statsd is a StatsD-compatible client for collecting operational
in-app metrics.

Supports counting, sampling, timing, gauges, sets and multi-metrics packet.

Using the client to increment a counter:

	client, err := statsd.Dial("127.0.0.1:8125")
	if err != nil {
		// handle error
	}
	defer client.Close()
	err = client.Increment("buckets", 1, 1)

*/
package statsd

import (
	"bufio"
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
	conn net.Conn
	buf  *bufio.Writer
	m    sync.Mutex
}

func millisecond(d time.Duration) int {
	return int(d.Seconds() * 1000)
}

// Dial connects to the given address on the given network using net.Dial and
// then returns a new client for the connection.
func Dial(addr string) (*client, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	return newClient(conn, 0), nil
}

// DialTimeout acts like Dial but takes a timeout. The timeout includes name
// resolution, if required.
func DialTimeout(addr string, timeout time.Duration) (*client, error) {
	conn, err := net.DialTimeout("udp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return newClient(conn, 0), nil
}

// DialSize acts like Dial but takes a packet size. By default, the packet size
// is 512, see
// https://github.com/etsy/statsd/blob/master/docs/metric_types.md#multi-metric-packets
// for guidelines.
func DialSize(addr string, size int) (*client, error) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return nil, err
	}
	return newClient(conn, size), nil
}

func newClient(conn net.Conn, size int) *client {
	if size <= 0 {
		size = defaultBufSize
	}
	return &client{
		conn: conn,
		buf:  bufio.NewWriterSize(conn, size),
	}
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
	return c.buf.Flush()
}

// Close implements the Client interface.
func (c *client) Close() error {
	if err := c.Flush(); err != nil {
		return err
	}
	c.buf = nil
	return c.conn.Close()
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

	// Flush data if we have reach the buffer limit
	if c.buf.Available() < len(format) {
		if err := c.Flush(); err != nil {
			return nil
		}
	}

	// Buffer is not empty, start filling it
	if c.buf.Buffered() > 0 {
		format = fmt.Sprintf("\n%s", format)
	}

	_, err := fmt.Fprintf(c.buf, format, args...)
	return err
}
