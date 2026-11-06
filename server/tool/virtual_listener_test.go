package tool

import (
	"io"
	"testing"
	"time"
)

func TestVirtualListenerDialAccept(t *testing.T) {
	l := NewVirtualListener(nil)
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		_, _ = c.Write([]byte("pong"))
		_ = c.Close()
	}()
	c, err := l.Dial(nil, "127.0.0.1:9")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	buf := make([]byte, 4)
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadFull(c, buf)
	if err != nil || n != 4 {
		t.Fatalf("read: n=%d err=%v", n, err)
	}
	if string(buf) != "pong" {
		t.Fatalf("got %q", buf)
	}
}

func TestDialBridgeAppRequiresListeners(t *testing.T) {
	SetWebServerListener(nil)
	if _, err := DialBridgeApp("web", "127.0.0.1:1"); err == nil {
		t.Fatal("expected web not running")
	}
	if _, err := DialBridgeApp("other", "127.0.0.1:1"); err == nil {
		t.Fatal("expected unsupported")
	}
	if _, err := DialBridgeApp("ipreg", "127.0.0.1:1"); err == nil {
		t.Fatal("expected ipreg unsupported")
	}
}

func TestDialBridgeAppWeb(t *testing.T) {
	web := NewVirtualListener(nil)
	defer web.Close()
	SetWebServerListener(web)
	defer SetWebServerListener(nil)

	go func() {
		c, err := web.Accept()
		if err != nil {
			return
		}
		_ = c.Close()
	}()
	c, err := DialBridgeApp("web", "203.0.113.9:9")
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}
