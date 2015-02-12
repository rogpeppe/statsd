package statsd

import (
	"log"
	"sync"
	"time"
)

var (
	current     Client = (*nilClient)(nil)
	clientMutex sync.Mutex
)

// SetAddr sets the network address that stats will be sent to.
func SetAddr(addr string) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	if current == nil {
		current = (*nilClient)(nil)
	}

	err := current.Close()
	if err != nil {
		log.Printf("error closing prior statsd connection: %v", err)
	}

	current, err = Dial(addr)
	if err != nil {
		log.Printf("error connecting to %q: %v", addr, err)
		current = (*nilClient)(nil)
	}

	return err
}

// Increment increments the counter for the given bucket.
func Increment(stat string, count int, rate float64) error {
	return current.Increment(stat, count, rate)
}

// Decrement decrements the counter for the given bucket.
func Decrement(stat string, count int, rate float64) error {
	return current.Decrement(stat, count, rate)
}

// Duration records time spent for the given bucket with time.Duration.
func Duration(stat string, duration time.Duration, rate float64) error {
	return current.Duration(stat, duration, rate)
}

// Timing records time spent for the given bucket in milliseconds.
func Timing(stat string, delta int, rate float64) error {
	return current.Timing(stat, delta, rate)
}

// Time calculates time spent in given function and send it.
func Time(stat string, rate float64, f func()) error {
	return current.Time(stat, rate, f)
}

// Gauge records arbitrary values for the given bucket.
func Gauge(stat string, value int, rate float64) error {
	return current.Gauge(stat, value, rate)
}

// IncrementGauge increments the value of the gauge.
func IncrementGauge(stat string, value int, rate float64) error {
	return current.IncrementGauge(stat, value, rate)
}

// DecrementGauge decrements the value of the gauge.
func DecrementGauge(stat string, value int, rate float64) error {
	return current.DecrementGauge(stat, value, rate)
}

// Unique records unique occurences of events.
func Unique(stat string, value int, rate float64) error {
	return current.Unique(stat, value, rate)
}
