package conn

import "testing"

func TestNewWebSocketStreamNil(t *testing.T) {
	if _, err := NewWebSocketStream(nil); err == nil {
		t.Fatal("expected error for nil websocket")
	}
}