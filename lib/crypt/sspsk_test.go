package crypt

import (
	"encoding/base64"
	"testing"
)

func TestNewShadowsocksPSK(t *testing.T) {
	psk := NewShadowsocksPSK()
	raw, err := base64.StdEncoding.DecodeString(psk)
	if err != nil || len(raw) != SSKeySize2022 {
		t.Fatalf("psk=%q len=%d err=%v", psk, len(raw), err)
	}
}

func TestNormalizeShadowsocksPSK(t *testing.T) {
	a := NewShadowsocksPSK()
	if got := NormalizeShadowsocksPSK(a); got != a {
		t.Fatalf("normalize valid want same, got %q", got)
	}
	if got := NormalizeShadowsocksPSK(""); got == "" {
		t.Fatal("empty should generate")
	}
	derived := NormalizeShadowsocksPSK("my-password")
	raw, err := base64.StdEncoding.DecodeString(derived)
	if err != nil || len(raw) != SSKeySize2022 {
		t.Fatalf("derived invalid: %v", err)
	}
}
