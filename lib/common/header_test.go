package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChangeHostAndHeaderSplitNAndRejectCRLF(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	ChangeHostAndHeader(r, "new.example", "X-Custom: a:b:c\nBad: v\r\ninject\nOk: 1", "1.2.3.4:5", false)
	if r.Host != "new.example" {
		t.Fatalf("host=%s", r.Host)
	}
	if got := r.Header.Get("X-Custom"); got != "a:b:c" {
		t.Fatalf("X-Custom=%q", got)
	}
	if r.Header.Get("Bad") != "" {
		t.Fatal("CRLF header must be rejected")
	}
	if got := r.Header.Get("Ok"); got != "1" {
		t.Fatalf("Ok=%q", got)
	}
}

func TestApplyHostCORS(t *testing.T) {
	r := httptest.NewRequest(http.MethodOptions, "http://example.com/", nil)
	r.Header.Set("Access-Control-Request-Headers", "X-Token")
	w := httptest.NewRecorder()
	ApplyHostCORS(w, r, "*")
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing ACAO")
	}
	if w.Header().Get("Access-Control-Allow-Headers") != "X-Token" {
		t.Fatal("missing ACAH")
	}
	ApplyHostCORS(w, r, "https://evil\r\nX-Inject: 1")
	if w.Header().Get("Access-Control-Allow-Origin") == "https://evil\r\nX-Inject: 1" {
		t.Fatal("CRLF origin must be rejected")
	}
}
