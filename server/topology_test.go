package server

import "testing"

func TestTopologyState(t *testing.T) {
	if topologyState(false, false, true) != "disabled" {
		t.Fatal("disabled")
	}
	if topologyState(true, true, true) != "running" {
		t.Fatal("running")
	}
	if topologyState(true, false, false) != "offline" {
		t.Fatal("offline")
	}
	if topologyState(true, false, true) != "stopped" {
		t.Fatal("stopped")
	}
}

func TestNetJoinPort(t *testing.T) {
	if got := netJoinPort("", 8080); got != "0.0.0.0:8080" {
		t.Fatalf("got %s", got)
	}
}
