package statsd

import (
	"time"
)

type nilClient struct{}

// Increment implements the Client interface.
func (*nilClient) Increment(stat string, count int, rate float64) error { return nil }

// Decrement implements the Client interface.
func (*nilClient) Decrement(stat string, count int, rate float64) error { return nil }

// Duration implements the Client interface.
func (*nilClient) Duration(stat string, duration time.Duration, rate float64) error { return nil }

// Timing implements the Client interface.
func (*nilClient) Timing(stat string, delta int, rate float64) error { return nil }

// Time implements the Client interface.
func (*nilClient) Time(stat string, rate float64, f func()) error { return nil }

// Gauge implements the Client interface.
func (*nilClient) Gauge(stat string, value int, rate float64) error { return nil }

// IncrementGauge implements the Client interface.
func (*nilClient) IncrementGauge(stat string, value int, rate float64) error { return nil }

// DecrementGauge implements the Client interface.
func (*nilClient) DecrementGauge(stat string, value int, rate float64) error { return nil }

// Unique implements the Client interface.
func (*nilClient) Unique(stat string, value int, rate float64) error { return nil }

// Flush implements the Client interface.
func (*nilClient) Flush() error { return nil }

// Close implements the Client interface.
func (*nilClient) Close() error { return nil }
