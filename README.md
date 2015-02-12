# statsd

statsd is a client library for statsd written in Go.

## Installation

Download and install :

```
$ go get gopkg.in/statsd.v1
```

Add it to your code :

```go
import "gopkg.in/statsd.v1"
```

## Use

```go
statsd.SetAddr("localhost:8125")
statsd.Increment("incr", 1, 1)
statsd.Decrement("decr", 1, 0.1)
statsd.Timing("timer", 320, 0.1)
statsd.Time("timer", 0.1, func() {
        // do something  
})
statsd.Gauge("gauge", 30, 1)
statsd.Unique("unique", 765, 1)
```
