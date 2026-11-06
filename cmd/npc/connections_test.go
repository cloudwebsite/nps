package main

import "testing"

func TestExpandNpcConnectionsSingle(t *testing.T) {
	got := expandNpcConnections("1.2.3.4:8024", "key", "tcp", false)
	if len(got) != 1 || got[0].server != "1.2.3.4:8024" || got[0].vkey != "key" || got[0].transport != "tcp" || got[0].tls {
		t.Fatalf("single = %#v", got)
	}
}

func TestExpandNpcConnectionsReuseLast(t *testing.T) {
	got := expandNpcConnections("a:1,b:2", "key", "tcp,tls", false)
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].server != "a:1" || got[0].vkey != "key" || got[0].transport != "tcp" || got[0].tls {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].server != "b:2" || got[1].vkey != "key" || got[1].transport != "tcp" || !got[1].tls {
		t.Fatalf("second = %#v", got[1])
	}
}

func TestExpandNpcConnectionsEmpty(t *testing.T) {
	if got := expandNpcConnections("", "k", "tcp", false); got != nil {
		t.Fatalf("want nil, got %#v", got)
	}
}
