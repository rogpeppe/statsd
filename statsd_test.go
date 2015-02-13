package statsd

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type testClient struct {
	client *client
	wg     sync.WaitGroup
	buf    bytes.Buffer
}

func newTestClient(t *testing.T) *testClient {
	r, w := net.Pipe()
	tc := &testClient{
		client: &client{
			size: defaultBufSize,
			conn: w,
		},
	}
	go func() {
		tc.wg.Add(1)
		defer tc.wg.Done()
		_, err := io.Copy(&tc.buf, r)
		if err != nil {
			t.Fatal(err)
		}
	}()
	return tc
}

func (tc *testClient) Close() error {
	tc.client.m.Lock()
	defer tc.client.m.Unlock()
	defer tc.client.conn.Close()
	return tc.client.flush()
}

func (tc *testClient) assertClose(t *testing.T) {
	err := tc.Close()
	if err != nil {
		t.Fatal(err)
	}
	tc.wg.Wait()
}

func assert(t *testing.T, value, control string) {
	if value != control {
		t.Errorf("incorrect command, want '%s', got '%s'", control, value)
	}
}

func TestIncrement(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.increment("incr", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "incr:1|c")
}

func TestDecrement(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.decrement("decr", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "decr:-1|c")
}

func TestDuration(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.duration("timing", time.Duration(123456789), 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "timing:123|ms")
}

func TestIncrementRate(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.increment("incr", 1, 0.99)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "incr:1|c|@0.99")
}

func TestPreciseRate(t *testing.T) {
	tc := newTestClient(t)
	// The real use case here is rates like 0.0001.
	err := tc.client.increment("incr", 1, 0.99901)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "incr:1|c|@0.99901")
}

func TestRate(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.increment("incr", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "")
}

func TestGauge(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.gauge("gauge", 300, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "gauge:300|g")
}

func TestIncrementGauge(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.incrementGauge("gauge", 10, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "gauge:+10|g")
}

func TestDecrementGauge(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.decrementGauge("gauge", 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "gauge:-4|g")
}

func TestUnique(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.unique("unique", 765, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "unique:765|s")
}

var millisecondTests = []struct {
	duration time.Duration
	control  int
}{{
	duration: 350 * time.Millisecond,
	control:  350,
}, {
	duration: 5 * time.Second,
	control:  5000,
}, {
	duration: 50 * time.Nanosecond,
	control:  0,
}}

func TestMilliseconds(t *testing.T) {
	for i, mt := range millisecondTests {
		value := millisecond(mt.duration)
		if value != mt.control {
			t.Errorf("%d: incorrect value, want %d, got %d", i, mt.control, value)
		}
	}
}

func TestTiming(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.timing("timing", 350, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "timing:350|ms")
}

func TestTime(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.time("time", 1, func() { time.Sleep(50e6) })
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultiPacket(t *testing.T) {
	tc := newTestClient(t)
	err := tc.client.unique("unique", 765, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = tc.client.unique("unique", 765, 1)
	if err != nil {
		t.Fatal(err)
	}
	tc.assertClose(t)
	assert(t, tc.buf.String(), "unique:765|s\nunique:765|s")
}

func TestMultiPacketOverflow(t *testing.T) {
	tc := newTestClient(t)
	for i := 0; i < 40; i++ {
		err := tc.client.unique("unique", 765, 1)
		if err != nil {
			t.Fatal(err)
		}
	}
	assert(t, tc.buf.String(), "unique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s\nunique:765|s")
	tc.buf.Reset()
	tc.assertClose(t)
	assert(t, tc.buf.String(), "unique:765|s")
}
