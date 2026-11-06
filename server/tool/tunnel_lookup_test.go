package tool

import (
	"net"
	"testing"
)

type stubDialer struct {
	remote string
}

func (s *stubDialer) DialVirtual(remote string) (net.Conn, error) {
	s.remote = remote
	a, b := net.Pipe()
	_ = b.Close()
	return a, nil
}

func TestGetTunnelConnDelegates(t *testing.T) {
	stub := &stubDialer{}
	SetTunnelLookup(func(id int) (TunnelDialer, bool) {
		if id != 7 {
			return nil, false
		}
		return stub, true
	})
	defer SetTunnelLookup(nil)

	c, err := GetTunnelConn(7, "1.2.3.4:5")
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if stub.remote != "1.2.3.4:5" {
		t.Fatalf("remote=%q", stub.remote)
	}
	if _, err := GetTunnelConn(8, "x"); err == nil {
		t.Fatal("expected missing tunnel error")
	}
}

func TestGetTunnelConnRequiresLookup(t *testing.T) {
	SetTunnelLookup(nil)
	if _, err := GetTunnelConn(1, "127.0.0.1:1"); err == nil {
		t.Fatal("expected not configured")
	}
	SetTunnelLookup(func(int) (TunnelDialer, bool) {
		return nil, false
	})
	defer SetTunnelLookup(nil)
	if _, err := GetTunnelConn(1, "127.0.0.1:1"); err == nil {
		t.Fatal("expected unavailable")
	}
}
