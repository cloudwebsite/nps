package bridge

import "testing"

func TestNestedTunnelOwnerAllowedSameClient(t *testing.T) {
	// 无 DB 时 GetTask 失败 → false；仅验证函数不会 panic。
	if nestedTunnelOwnerAllowed(1, 999999) {
		t.Fatal("missing task must deny")
	}
}

func TestResolveFileAccessTaskEmpty(t *testing.T) {
	if _, err := resolveFileAccessTask(""); err == nil {
		t.Fatal("empty file:// must fail")
	}
}
