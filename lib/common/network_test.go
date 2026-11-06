package common

import (
	"net/http"
	"testing"
)

func TestEffectiveHTTPClientIP(t *testing.T) {
	h := http.Header{}
	h.Set("X-Real-IP", "203.0.113.9")
	h.Set("X-Forwarded-For", "198.51.100.4, 172.18.0.2")
	if got := EffectiveHTTPClientIP("172.18.0.2:43122", h, false); got != "172.18.0.2" {
		t.Fatalf("forwarded disabled = %q", got)
	}
	if got := EffectiveHTTPClientIP("172.18.0.2:43122", h, true); got != "203.0.113.9" {
		t.Fatalf("prefer X-Real-IP = %q", got)
	}

	h2 := http.Header{}
	h2.Set("X-Real-IP", "not-an-ip")
	h2.Set("X-Forwarded-For", "198.51.100.4, 172.18.0.2")
	if got := EffectiveHTTPClientIP("172.18.0.2:43122", h2, true); got != "198.51.100.4" {
		t.Fatalf("fallback X-Forwarded-For = %q", got)
	}
	if got := EffectiveHTTPClientIP("172.18.0.2:43122", http.Header{}, true); got != "172.18.0.2" {
		t.Fatalf("peer fallback = %q", got)
	}
}

func TestEffectiveHTTPClientIPCDNHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("CF-Connecting-IP", "203.0.113.50")
	h.Set("X-Real-IP", "198.51.100.1")
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h, true); got != "203.0.113.50" {
		t.Fatalf("CF-Connecting-IP first = %q", got)
	}

	h2 := http.Header{}
	h2.Set("Ali-Cdn-Real-IP", "203.0.113.60")
	h2.Set("X-Forwarded-For", "198.51.100.2")
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h2, true); got != "203.0.113.60" {
		t.Fatalf("Ali-Cdn-Real-IP = %q", got)
	}

	h3 := http.Header{}
	h3.Set("EO-Connecting-IP", "203.0.113.70")
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h3, true); got != "203.0.113.70" {
		t.Fatalf("EO-Connecting-IP = %q", got)
	}
}

func TestEffectiveHTTPClientIPForwarded(t *testing.T) {
	h := http.Header{}
	h.Set("Forwarded", `for=203.0.113.80;proto=https;by=172.18.0.2`)
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h, true); got != "203.0.113.80" {
		t.Fatalf("Forwarded for= = %q", got)
	}

	h2 := http.Header{}
	h2.Set("Forwarded", `for="[2001:db8::1]:4711";proto=http`)
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h2, true); got != "2001:db8::1" {
		t.Fatalf("Forwarded IPv6 = %q", got)
	}

	h3 := http.Header{}
	h3.Set("Forwarded", `for=unknown;proto=http`)
	if got := EffectiveHTTPClientIP("172.18.0.2:1", h3, true); got != "172.18.0.2" {
		t.Fatalf("Forwarded unknown fallback = %q", got)
	}
}
