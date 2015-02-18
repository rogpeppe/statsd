/*
Package statsd is a StatsD-compatible client for collecting operational
in-app metrics.

It supports counting, sampling, timing, gauges, sets and multi-metrics packet.

Example usage:

	err := statsd.SetAddr("127.0.0.1:8125")
	if err != nil {
		// handle error
	}
	err = statsd.Increment("buckets", 1, 1)

Where there is a rate parameter, it should be a number between
0 and 1 holding the probability of the event being logged.
If it is one, the event will always be logged.
*/
package statsd

import (
	"errors"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	defaultBufSize = 512
)

// Client represents a statsd client.
type Client struct {
	size int

	// mu guards the following fields.
	mu        sync.Mutex
	rand      *rand.Rand
	conn      io.WriteCloser
	buf       []byte
	errorFunc func(error)
}

// NewClient creates a new statsd client that
// will send stats to the given UDP host and port.
func NewClient(hostPort string) (*Client, error) {
	c := &Client{
		size: defaultBufSize,
		rand: rand.New(rand.NewSource(0)),
	}
	if err := c.SetHostPort(hostPort); err != nil {
		return nil, err
	}
	return c, nil
}

// SetErrorFunc sets a function that will be called
// when any error occurs when writing stats data.
// The function should not block, and in particular
// it must not operate on the same Client, as that
// will cause a deadlock.
//
// If f is nil (the default for a new client), errors
// will be ignored.
func (c *Client) SetErrorFunc(f func(err error)) {
	c.mu.Lock()
	c.errorFunc = f
	c.mu.Unlock()
}

// SetHostPort sets the UDP addressto which stats
// will be sent. If it returns an error, the address will
// remain unchanged.
func (c *Client) SetHostPort(addr string) error {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	c.mu.Unlock()
	return nil
}

// Increment causes the given counter statistic to be incremented
// by the given delta. To decrement a counter, use a negative
// delta.
func (c *Client) Increment(stat string, delta int, rate float64) {
	c.send(&metric{
		kind: "c",
		stat: stat,
		rate: rate,
		n:    delta,
	})
}

// Duration records a duration value for the given statistic.
// The duration is assumed to be non-negative
// and is rounded to the nearest millisecond.
func (c *Client) Duration(stat string, duration time.Duration, rate float64) {
	c.send(&metric{
		kind: "ms",
		stat: stat,
		rate: rate,
		n:    int((duration + time.Millisecond/2) / time.Millisecond),
	})
}

// TimeFunction calls f and records a duration value for
// the given statistic recording the length of time it took
// to run. The function will always be called even if the
// rate is less than 1.
func (c *Client) TimeFunction(stat string, rate float64, f func()) {
	ts := time.Now()
	f()
	c.Duration(stat, time.Since(ts), rate)
}

// Gauge records an absolute value for the given stat.
func (c *Client) Gauge(stat string, value int) {
	c.send(&metric{
		kind: "g",
		sign: signNone,
		stat: stat,
		rate: 1,
		n:    value,
	})
}

// IncrementGauge changes the value of the gauge
// by the given delta. To decrement a gauge, use a negative
// delta.
func (c *Client) IncrementGauge(stat string, delta int) {
	c.send(&metric{
		kind: "g",
		sign: signRequired,
		stat: stat,
		rate: 1,
		n:    delta,
	})
}

// Unique records that the given count of unique statevents
// have occurred occurred.
func (c *Client) Unique(stat string, count int) {
	c.send(&metric{
		kind: "s",
		stat: stat,
		n:    count,
		rate: 1,
	})
}

// flush writes all buffered stats messages to the client connection.
// The caller must hold the client mutex lock.
func (c *Client) flush() {
	if len(c.buf) == 0 {
		return
	}
	_, err := c.conn.Write(c.buf)
	c.buf = c.buf[:0]
	if err != nil && c.errorFunc != nil {
		c.errorFunc(err)
	}
}

// Flush flushes all buffered statistics.
// TODO do this automatically after some
// time has elapsed since the last statistic.
func (c *Client) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flush()
}

type sign uint8

const (
	signMaybe sign = iota
	signRequired
	signNone
)

type metric struct {
	stat string
	sign sign
	n    int
	kind string
	rate float64
}

var errTooBig = errors.New("metric too big")

func (c *Client) send(m *metric) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m.rate < 1 && c.rand.Float64() >= m.rate {
		return
	}
	oldLen := len(c.buf)
	buf := m.append(c.buf)
	if len(buf) <= c.size {
		c.buf = buf
		return
	}
	if oldLen == 0 {
		if c.errorFunc != nil {
			c.errorFunc(errTooBig)
		}
		return
	}
	c.flush()
	// Copy the recently appended data to the start
	// of the buffer, omitting the initial newline.
	c.buf = append(c.buf, buf[oldLen+1:]...)
}

// append appends the metric data to the given
// buffer and returns the new buffer.
func (m *metric) append(buf []byte) []byte {
	if len(buf) > 0 {
		buf = append(buf, '\n')
	}
	if m.sign == signNone && m.n < 0 {
		// We're trying to send a negative absolute value
		// for a gauge. We can't do that without resetting
		// the value first.
		buf = append(buf, m.stat...)
		buf = append(buf, ":0|g\n"...)
	}
	buf = append(buf, m.stat...)
	buf = append(buf, ':')
	if m.sign == signRequired && m.n >= 0 {
		buf = append(buf, '+')
	}
	buf = strconv.AppendInt(buf, int64(m.n), 10)
	buf = append(buf, '|')
	buf = append(buf, m.kind...)
	if m.rate < 1 {
		buf = append(buf, '|', '@')
		buf = strconv.AppendFloat(buf, m.rate, 'f', -1, 64)
	}
	return buf
}
