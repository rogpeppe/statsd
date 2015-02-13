package statsd

import "time"

var (
	defaultClient *client = newClient()
)

// SetAddr sets the network address that stats will be sent to.
func SetAddr(addr string) error {
	return defaultClient.setAddr(addr)
}

// Increment increments the counter for the given bucket.
func Increment(stat string, count int, rate float64) error {
	return defaultClient.increment(stat, count, rate)
}

// Decrement decrements the counter for the given bucket.
func Decrement(stat string, count int, rate float64) error {
	return defaultClient.decrement(stat, count, rate)
}

// Duration records time spent for the given bucket with time.Duration.
func Duration(stat string, duration time.Duration, rate float64) error {
	return defaultClient.duration(stat, duration, rate)
}

// Timing records time spent for the given bucket in milliseconds.
func Timing(stat string, delta int, rate float64) error {
	return defaultClient.timing(stat, delta, rate)
}

// Time calculates time spent in given function and send it.
func Time(stat string, rate float64, f func()) error {
	return defaultClient.time(stat, rate, f)
}

// Gauge records arbitrary values for the given bucket.
func Gauge(stat string, value int, rate float64) error {
	return defaultClient.gauge(stat, value, rate)
}

// IncrementGauge increments the value of the gauge.
func IncrementGauge(stat string, value int, rate float64) error {
	return defaultClient.incrementGauge(stat, value, rate)
}

// DecrementGauge decrements the value of the gauge.
func DecrementGauge(stat string, value int, rate float64) error {
	return defaultClient.decrementGauge(stat, value, rate)
}

// Unique records unique occurences of events.
func Unique(stat string, value int, rate float64) error {
	return defaultClient.unique(stat, value, rate)
}

// Flush writes any buffered data to the network.
func Flush() error {
	defaultClient.m.Lock()
	defer defaultClient.m.Unlock()
	return defaultClient.flush()
}
