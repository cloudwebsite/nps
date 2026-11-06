package main

import (
	"encoding/base64"
	"testing"
)

func TestParseShortcutBase64Legacy(t *testing.T) {
	raw := "nps:MyServer|127.0.0.1:8024|mykey123|false"
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	sc, err := parseShortcutBase64(enc)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Name != "MyServer" || sc.Addr != "127.0.0.1:8024" || sc.Key != "mykey123" || sc.TLS {
		t.Fatalf("unexpected legacy parse: %+v", sc)
	}
	if sc.P2PMode != "legacy" {
		t.Fatalf("default p2pMode want legacy, got %q", sc.P2PMode)
	}
}

func TestParseShortcutBase64Extended(t *testing.T) {
	raw := "nps:Srv|a:8024|k|true|modern|aabbcc|nps:7000"
	enc := base64.StdEncoding.EncodeToString([]byte(raw))
	sc, err := parseShortcutBase64(enc)
	if err != nil {
		t.Fatal(err)
	}
	if !sc.TLS || sc.P2PMode != "modern" || sc.TlsFingerprint != "aabbcc" || sc.P2PHolepunch != "nps:7000" {
		t.Fatalf("unexpected extended parse: %+v", sc)
	}
}
