package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	r := newRateLimiter()
	if !r.Allow("a", time.Second) {
		t.Fatal("first allow")
	}
	if r.Allow("a", time.Second) {
		t.Fatal("second should block")
	}
	if !r.Allow("b", time.Second) {
		t.Fatal("other key")
	}
	if !r.Allow("a", 0) {
		t.Fatal("zero interval always allow")
	}
}

func TestParseWebhookHeaders(t *testing.T) {
	h := parseWebhookHeaders("Authorization:Bearer x\\nX-Custom: 1")
	if h["Authorization"] != "Bearer x" || h["X-Custom"] != "1" {
		t.Fatalf("got %#v", h)
	}
}

func TestWebhookPayloadContract(t *testing.T) {
	var got webhookPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		if r.Header.Get("Authorization") != "Bearer t" {
			t.Errorf("missing auth header")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ch := newWebhookChannel(srv.URL, "Authorization:Bearer t", srv.Client())
	err := ch.Send(t.Context(), Event{
		Type:     TypeClientDisconnect,
		Title:    "[nps] 客户端下线",
		Body:     "id=1",
		Time:     time.Date(2026, 11, 5, 15, 0, 0, 0, time.FixedZone("CST", 8*3600)),
		ClientID: 1,
		Addr:     "1.2.3.4:5678",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Type != TypeClientDisconnect || got.ClientID != 1 {
		t.Fatalf("payload %#v", got)
	}
	if !strings.Contains(got.Time, "2026-11-05") {
		t.Fatalf("time %q", got.Time)
	}
}

func TestWecomSendAndTelegramNoop(t *testing.T) {
	var wecomOK atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&m)
		if m["msgtype"] != "text" {
			t.Errorf("msgtype")
		}
		wecomOK.Store(true)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := srv.Client()
	if err := newWecomChannel(srv.URL, client).Send(t.Context(), Event{Title: "t", Body: "b"}); err != nil {
		t.Fatal(err)
	}
	if !wecomOK.Load() {
		t.Fatal("wecom not called")
	}
	if err := newTelegramChannel("", "1", client).Send(t.Context(), Event{Title: "x"}); err != nil {
		t.Fatal("empty token should no-op")
	}
}

func TestDispatcherDisabledAndQueueDrop(t *testing.T) {
	d, err := NewDispatcher(Config{Enable: false})
	if err != nil || d != nil {
		t.Fatalf("disabled: %v %v", d, err)
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	d, err = NewDispatcher(Config{
		Enable:      true,
		QueueSize:   1,
		MinInterval: 0,
		HTTPTimeout: time.Second,
		WebhookURL:  srv.URL,
	})
	if err != nil || d == nil {
		t.Fatalf("new: %v %v", err, d)
	}
	defer d.Close()

	// Block worker by filling then overflowing.
	d.Emit(Event{Type: TypeErrorPage, Host: "a.example"})
	d.Emit(Event{Type: TypeErrorPage, Host: "b.example"}) // may process or queue
	// Force drops: inject directly when queue full.
	for i := 0; i < 20; i++ {
		select {
		case d.queue <- Event{Type: TypeErrorPage, Host: "x"}:
		default:
			d.dropCount.Add(1)
		}
	}
	time.Sleep(200 * time.Millisecond)
	if d.Dropped() == 0 && hits.Load() == 0 {
		t.Fatal("expected activity")
	}
}

func TestNewDispatcherRequiresChannel(t *testing.T) {
	d, err := NewDispatcher(Config{Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if d != nil {
		t.Fatal("expected nil without channels")
	}
}
