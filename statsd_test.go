package statsd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

type testClient struct {
	*Client
	buf bytes.Buffer
}

func newTestClient() *testClient {
	c, err := NewClient("127.0.0.1:999")
	if err != nil {
		panic(fmt.Errorf("cannot make new client: %v", err))
	}
	c.SetErrorFunc(func(err error) {
		panic(fmt.Errorf("unexpected error: %v", err))
	})
	tc := &testClient{
		Client: c,
	}
	tc.conn.Close()
	tc.conn = nopCloser{&tc.buf}
	return tc
}

func assert(t *testing.T, value, control string) {
	if value != control {
		t.Errorf("incorrect command, want '%s', got '%s'", control, value)
	}
}

func TestNewClientWithError(t *testing.T) {
	c, err := NewClient("noport")
	want := "dial udp: missing port in address noport"
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected error; got %v want %q", err, want)
	}
	if c != nil {
		t.Fatalf("unexpected non-nil return with error")
	}
}

func TestSetHostPort(t *testing.T) {
	hostPort0, packetc0 := listener()
	hostPort1, packetc1 := listener()
	c, err := NewClient(hostPort0)
	if err != nil {
		t.Fatal(err)
	}
	c.Increment("a", 1, 1)
	c.Increment("b", 1, 1)
	c.Flush()
	want := "a:1|c\nb:1|c"
	if got := <-packetc0; got != want {
		t.Errorf("unexpected packet; got %q want %q", got, want)
	}
	err = c.SetHostPort(hostPort1)
	if err != nil {
		t.Fatal(err)
	}
	c.Increment("c", 1, 1)
	c.Increment("d", 1, 1)
	c.Flush()
	want = "c:1|c\nd:1|c"
	if got := <-packetc1; got != want {
		t.Errorf("unexpected packet; got %q want %q", got, want)
	}
}

func TestWriteError(t *testing.T) {
	c, err := NewClient("127.0.0.1:999")
	if err != nil {
		panic(fmt.Errorf("cannot make new client: %v", err))
	}
	c.conn = nopCloser{errorWriter("testing write error")}
	c.Increment("a", 1, 1)
	c.Flush()
}

func TestErrorFunc(t *testing.T) {
	c, err := NewClient("127.0.0.1:999")
	if err != nil {
		panic(fmt.Errorf("cannot make new client: %v", err))
	}
	var gotErr error
	c.SetErrorFunc(func(err error) {
		gotErr = err
	})
	errStr := "testing write error"
	c.conn = nopCloser{errorWriter(errStr)}
	c.Increment("a", 1, 1)
	c.Flush()
	if gotErr == nil || gotErr.Error() != errStr {
		t.Errorf("error func not called or unexpected error; got %v want %v", gotErr, errStr)
	}
}

func TestNoWriteOnEmptyFlush(t *testing.T) {
	tc := newTestClient()
	gotError := false
	tc.SetErrorFunc(func(err error) {
		gotError = true
	})
	tc.conn = nopCloser{errorWriter("testing write error")}
	tc.Flush()
	if gotError {
		t.Error("buffer was flushed on empty")
	}
	// Sanity check that we do see the error when there is something in the buffer.
	tc.Increment("a", 1, 1)
	tc.Flush()
	if !gotError {
		t.Errorf("expected an error, but found none")
	}
}

func listener() (hostPort string, _ <-chan string) {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	packetc := make(chan string, 1)
	go func() {
		buf := make([]byte, defaultBufSize)
		for {
			n, _, err := c.ReadFrom(buf)
			if err != nil {
				panic(err)
			}
			packetc <- string(buf[0:n])
		}
	}()
	return c.LocalAddr().String(), packetc
}

func TestIncrement(t *testing.T) {
	tc := newTestClient()
	tc.Increment("incr", 99, 1)
	tc.Flush()
	assert(t, tc.buf.String(), "incr:99|c")
}

func TestDecrement(t *testing.T) {
	tc := newTestClient()
	tc.Increment("decr", -99, 1)
	tc.Flush()
	assert(t, tc.buf.String(), "decr:-99|c")
}

func TestDuration(t *testing.T) {
	tc := newTestClient()
	tc.Duration("t", time.Duration(123000000), 1)
	tc.Duration("t", time.Duration(123123456), 1)
	tc.Duration("t", time.Duration(123499999), 1)
	tc.Duration("t", time.Duration(123500000), 1)
	tc.Duration("t", time.Duration(123999999), 1)
	tc.Duration("t", time.Duration(124000000), 1)
	tc.Flush()
	assert(t, tc.buf.String(), strings.Join([]string{
		"t:123|ms",
		"t:123|ms",
		"t:123|ms",
		"t:124|ms",
		"t:124|ms",
		"t:124|ms",
	}, "\n"))
}

func TestIncrementRate(t *testing.T) {
	tc := newTestClient()
	tc.Increment("incr", 1, 0.99)
	tc.Flush()
	assert(t, tc.buf.String(), "incr:1|c|@0.99")
}

func TestRate(t *testing.T) {
	// Note that this test is deterministic because the client
	// always starts with the same random number seed.

	tc := newTestClient()
	var cw countingWriter
	tc.conn = nopCloser{&cw}
	for i := 0; i < 100000; i++ {
		tc.Increment("x", 1, 0.456)
	}
	tc.Flush()

	if cw.n < 45500 || cw.n > 45700 {
		t.Errorf("logging rate outside expected bounds; got %d want ~45600", cw.n)
	}
}

func TestPreciseRate(t *testing.T) {
	// Note that this test is deterministic because the client
	// always starts with the same random number seed.

	tc := newTestClient()
	// The real use case here is rates like 0.0001.
	tc.Increment("incr", 1, 0.99901)
	tc.Flush()
	assert(t, tc.buf.String(), "incr:1|c|@0.99901")
}

func TestStatTooBig(t *testing.T) {
	tc := newTestClient()
	var gotErr error
	tc.SetErrorFunc(func(err error) {
		gotErr = err
	})
	tc.Increment(strings.Repeat("a", tc.size), 1, 1)
	if gotErr != errTooBig {
		t.Errorf("unexpected error; expected %v got %v", errTooBig, gotErr)
	}
}

func TestVerySmallRate(t *testing.T) {
	// We can't easily test a very small rate because
	// it's unlikely to happen, so test the underlying
	// metric append logic instead.
	m := &metric{
		stat: "incr",
		n:    1,
		kind: "c",
		rate: 0.0000000001,
	}
	buf := m.append(nil)
	assert(t, string(buf), "incr:1|c|@0.0000000001")
}

func TestZeroRate(t *testing.T) {
	tc := newTestClient()
	tc.Increment("incr", 1, 0)
	tc.Flush()
	assert(t, tc.buf.String(), "")
}

func TestGauge(t *testing.T) {
	tc := newTestClient()
	tc.Gauge("gauge", 300)
	tc.Flush()
	assert(t, tc.buf.String(), "gauge:300|g")
}

func TestNegativeGauge(t *testing.T) {
	tc := newTestClient()
	tc.Gauge("gauge", -300)
	tc.Flush()
	assert(t, tc.buf.String(), "gauge:0|g\ngauge:-300|g")
}

func TestIncrementGauge(t *testing.T) {
	tc := newTestClient()
	tc.IncrementGauge("gauge", 10)
	tc.Flush()
	assert(t, tc.buf.String(), "gauge:+10|g")
}

func TestDecrementGauge(t *testing.T) {
	tc := newTestClient()
	tc.IncrementGauge("gauge", -4)
	tc.Flush()
	assert(t, tc.buf.String(), "gauge:-4|g")
}

func TestUnique(t *testing.T) {
	tc := newTestClient()
	tc.Unique("unique", 765)
	tc.Flush()
	assert(t, tc.buf.String(), "unique:765|s")
}

func TestTimeFunction(t *testing.T) {
	tc := newTestClient()
	tc.TimeFunction("time", 1, func() { time.Sleep(2 * time.Millisecond) })
	tc.Flush()
	var n int
	_, err := fmt.Sscanf(tc.buf.String(), "time:%d|ms", &n)
	if err != nil {
		t.Fatalf("cannot scan %q: %v", tc.buf.String(), err)
	}
	if n < 2 {
		t.Fatalf("test should have waited for at least 2ms, got %d", n)
	}
}

func TestMultiPacket(t *testing.T) {
	tc := newTestClient()
	tc.Unique("unique", 765)
	tc.Unique("unique", 765)
	tc.Flush()
	assert(t, tc.buf.String(), "unique:765|s\nunique:765|s")
}

func TestMultiPacketOverflow(t *testing.T) {
	tc := newTestClient()
	var w writeRecorder
	tc.conn = &w
	msg := "unique:765|s"
	tc.Unique("unique", 765)
	if len(w.data) != 0 {
		t.Fatal("got %d writes, expected none", len(w.data))
	}
	totalBytes := len(msg)
	count := 0
	for {
		totalBytes += len(msg) + 1
		if totalBytes > tc.size {
			break
		}
		count++
		tc.Unique("unique", 765)
		if len(w.data) != 0 {
			t.Fatal("got %d writes, expected none", len(w.data))
		}
	}
	tc.Unique("unique", 765)
	want := []string{
		msg + strings.Repeat("\n"+msg, count),
	}
	if !reflect.DeepEqual(w.data, want) {
		t.Fatal("got %#v want %#v", w.data, want)
	}
	w.data = nil
	tc.Flush()
	want = []string{msg}
	if !reflect.DeepEqual(w.data, want) {
		t.Fatal("got %#v want %#v", w.data, want)
	}
}

func BenchmarkIncrement(b *testing.B) {
	tc := newTestClient()
	tc.conn = nopWriter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc.Increment("stat", 1, 1)
	}
}

type nopWriter struct{}

func (nopWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func (nopWriter) Close() error {
	return nil
}

type writeRecorder struct {
	data []string
}

func (w *writeRecorder) Write(buf []byte) (int, error) {
	w.data = append(w.data, string(buf))
	return len(buf), nil
}

func (w *writeRecorder) Close() error {
	return nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

type errorWriter string

func (w errorWriter) Write([]byte) (int, error) {
	return 0, errors.New(string(w))
}

// countingWriter counts the number stats sent to it.
type countingWriter struct {
	n int
}

var nl = []byte("\n")

func (w *countingWriter) Write(buf []byte) (int, error) {
	w.n += bytes.Count(buf, nl) + 1
	return len(buf), nil
}
