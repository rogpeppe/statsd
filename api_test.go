package statsd

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestSetAddr(t *testing.T) {
	ln, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	addr := ln.LocalAddr()
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		t.Fatal("not a UDP address")
	}

	t.Logf("listening on %s", addr)
	clientAddr := fmt.Sprintf("localhost:%d", udpAddr.Port)
	t.Logf("connecting to %s", clientAddr)

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
