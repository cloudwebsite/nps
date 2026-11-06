package controllers

import "testing"

func TestValidateTunnelInputModeWhitelist(t *testing.T) {
	if msg := validateTunnelInput("webServer", "1.1.1.1:80", "", ""); msg == "" {
		t.Fatal("internal mode must be rejected")
	}
	if msg := validateTunnelInput("tcp", "", "", ""); msg == "" {
		t.Fatal("tcp requires target")
	}
	if msg := validateTunnelInput("tcp", "127.0.0.1:22", "", ""); msg != "" {
		t.Fatalf("ok tcp: %s", msg)
	}
	if msg := validateTunnelInput("socks5", "", "", ""); msg != "" {
		t.Fatalf("ok socks5: %s", msg)
	}
	if msg := validateTunnelInput("mixProxy", "", "", ""); msg != "" {
		t.Fatalf("ok mixProxy: %s", msg)
	}
}
