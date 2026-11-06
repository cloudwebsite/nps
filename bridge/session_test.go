package bridge

import (
	"testing"

	"ehang.io/nps/lib/nps_mux"
)

func TestPickTunnelRoundRobin(t *testing.T) {
	_ = SetClientSelectMode("roundrobin")
	defer func() { _ = SetClientSelectMode("primary") }()

	a := &nps_mux.Mux{}
	b := &nps_mux.Mux{}
	c := &Client{tunnels: []*nps_mux.Mux{a, b}, tunnel: b}
	first := c.pickTunnelLocked()
	second := c.pickTunnelLocked()
	if first == nil || second == nil || first == second {
		t.Fatalf("round-robin expected alternating picks, got %p %p", first, second)
	}
}

func TestAddTunnelPrimaryReplaces(t *testing.T) {
	_ = SetClientSelectMode("primary")
	old := &nps_mux.Mux{}
	neu := &nps_mux.Mux{}
	c := &Client{tunnel: old, tunnels: []*nps_mux.Mux{old}}
	toClose := c.addTunnelLocked(neu)
	if len(toClose) != 1 || toClose[0] != old {
		t.Fatalf("primary should close old tunnel, got %#v", toClose)
	}
	if c.tunnel != neu || len(c.tunnels) != 1 {
		t.Fatalf("primary should keep single tunnel")
	}
}

func TestAddTunnelRoundRobinKeepsBoth(t *testing.T) {
	_ = SetClientSelectMode("roundrobin")
	defer func() { _ = SetClientSelectMode("primary") }()
	old := &nps_mux.Mux{}
	neu := &nps_mux.Mux{}
	c := &Client{tunnel: old, tunnels: []*nps_mux.Mux{old}}
	toClose := c.addTunnelLocked(neu)
	if len(toClose) != 0 {
		t.Fatalf("roundrobin should keep both, close=%v", toClose)
	}
	if len(c.tunnels) != 2 {
		t.Fatalf("want 2 tunnels, got %d", len(c.tunnels))
	}
}
