package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSameSiteHandlerAddsStrict(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "beegosessionID", Value: "abc", Path: "/"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	newSameSiteHandler(inner, false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	got := rec.Header().Get("Set-Cookie")
	if !strings.Contains(got, "SameSite=Strict") {
		t.Fatalf("missing SameSite: %q", got)
	}
	if strings.Contains(got, "Secure") {
		t.Fatalf("http should not force Secure: %q", got)
	}
}

func TestSameSiteHandlerAddsSecureOnHTTPS(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "beegosessionID", Value: "abc", Path: "/"})
		_, _ = w.Write([]byte("ok"))
	})
	rec := httptest.NewRecorder()
	newSameSiteHandler(inner, true).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	got := rec.Header().Get("Set-Cookie")
	if !strings.Contains(got, "SameSite=Strict") || !strings.Contains(got, "Secure") {
		t.Fatalf("want SameSite+Secure, got %q", got)
	}
}
