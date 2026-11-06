//go:build !npcgui
// +build !npcgui

package proxy

import (
	"testing"

	"ehang.io/nps/lib/file"
)

func TestLooksLikeHTTPProxyRequest(t *testing.T) {
	if !looksLikeHTTPProxyRequest([]byte("GE")) {
		t.Fatal("GET prefix")
	}
	if !looksLikeHTTPProxyRequest([]byte("CO")) {
		t.Fatal("CONNECT prefix")
	}
	if looksLikeHTTPProxyRequest([]byte{5, 1}) {
		t.Fatal("socks5 must not look like HTTP")
	}
}

func TestEffectiveMixProxyFlags(t *testing.T) {
	httpOn, socksOn := effectiveMixProxyFlags(&file.Tunnel{Mode: "mixProxy"})
	if !httpOn || !socksOn {
		t.Fatalf("mixProxy empty flags should default both on, got %v %v", httpOn, socksOn)
	}
	httpOn, socksOn = effectiveMixProxyFlags(&file.Tunnel{Mode: "mixProxy", HttpProxy: true})
	if !httpOn || socksOn {
		t.Fatalf("only http: got %v %v", httpOn, socksOn)
	}
	httpOn, socksOn = effectiveMixProxyFlags(&file.Tunnel{Mode: "httpProxy"})
	if !httpOn || socksOn {
		t.Fatalf("httpProxy mode: got %v %v", httpOn, socksOn)
	}
	httpOn, socksOn = effectiveMixProxyFlags(&file.Tunnel{Mode: "socks5"})
	if httpOn || !socksOn {
		t.Fatalf("socks5 mode: got %v %v", httpOn, socksOn)
	}
}
