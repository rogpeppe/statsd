package statsd

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func assertUDPPort(t *testing.T, ln net.PacketConn) int {
	addr := ln.LocalAddr()
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		t.Fatalf("not a UDP addr: %v", addr)
	}
	return udpAddr.Port
}

func TestSetAddr(t *testing.T) {
	ln, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	port := assertUDPPort(t, ln)
	clientAddr := fmt.Sprintf("localhost:%d", port)

	err = SetAddr(clientAddr)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan struct{})
	out := make([]byte, 512)
	go func() {
		n, _, err := ln.ReadFrom(out)
		if err != nil {
			t.Fatal(err)
		}
		out = out[:n]
		close(ch)
	}()
	err = Increment("incr", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = Flush()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}

	if string(out) != "incr:1|c" {
		t.Fatalf("unexpected output: %q", string(out))
	}
}

func TestReconnect(t *testing.T) {
	// Acquire a port and close it, just to get an unused port.
	ln, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := assertUDPPort(t, ln)
	err = ln.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Try sending a bunch of stats with no server listening.
	err = SetAddr(fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatal(err)
	}

	// Metrics get dropped on the floor.
	for i := 0; i < 1000; i++ {
		Gauge("novelty", i, 1)
	}
	Flush()

	// Now start a server that is listening on the configured port.
	ln, err = net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ch := make(chan struct{})
	out := make([]byte, 512)
	go func() {
		n, _, err := ln.ReadFrom(out)
		if err != nil {
			t.Fatal(err)
		}
		out = out[:n]
		close(ch)
	}()
	err = Increment("incr", 1001, 1)
	if err != nil {
		t.Fatal(err)
	}
	err = Flush()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout")
	}

	if string(out) != "incr:1001|c" {
		t.Fatalf("unexpected output: %q", string(out))
	}
}
