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
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	defaultBufSize = 512
)

type client struct {
	size int

	m    sync.Mutex
	addr string
	conn net.Conn
	buf  bytes.Buffer
}

func millisecond(d time.Duration) int {
	return int(d.Seconds() * 1000)
}

func newClient() *client {
	return &client{
		size: defaultBufSize,
	}
}

// setAddr connects the client to a new address, to which stats will be sent.
func (c *client) setAddr(addr string) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.addr = addr
	return c.connect()
}

// connect dials the currently configured address. Caller must hold the client
// mutex lock. This method either returns an error, or sets the client
// connection.
func (c *client) connect() error {
	var err error

	if c.addr == "" {
		return errors.New("address not set")
	}

	if c.conn != nil {
		err = c.conn.Close()
		if err != nil {
			log.Printf("error closing prior client connection: %v", err)
		}
	}

	conn, err := net.Dial("udp", c.addr)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *client) increment(stat string, count int, rate float64) error {
	return c.send(stat, rate, "%d|c", count)
}

func (c *client) decrement(stat string, count int, rate float64) error {
	return c.increment(stat, -count, rate)
}

func (c *client) duration(stat string, duration time.Duration, rate float64) error {
	return c.send(stat, rate, "%d|ms", millisecond(duration))
}

func (c *client) timing(stat string, delta int, rate float64) error {
	return c.send(stat, rate, "%d|ms", delta)
}

func (c *client) time(stat string, rate float64, f func()) error {
	ts := time.Now()
	f()
	return c.duration(stat, time.Since(ts), rate)
}

func (c *client) gauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "%d|g", value)
}

func (c *client) incrementGauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "+%d|g", value)
}

func (c *client) decrementGauge(stat string, value int, rate float64) error {
	return c.send(stat, rate, "-%d|g", value)
}

func (c *client) unique(stat string, value int, rate float64) error {
	return c.send(stat, rate, "%d|s", value)
}

// flush writes all buffered stats messages to the client connection. Caller
// must hold the client mutex lock.
func (c *client) flush() error {
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
		err = c.flush()
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
