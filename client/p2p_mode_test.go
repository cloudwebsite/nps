package client

import "testing"

func TestSetP2PMode(t *testing.T) {
	if err := SetP2PMode("stun"); err != nil {
		t.Fatal(err)
	}
	if GetP2PMode() != P2PModeSTUN || P2PModeName() != "stun" {
		t.Fatalf("want stun")
	}
	if err := SetP2PMode("modern"); err != nil {
		t.Fatal(err)
	}
	if GetP2PMode() != P2PModeModern || P2PModeName() != "modern" {
		t.Fatalf("want modern")
	}
	if err := SetP2PMode("bad"); err == nil {
		t.Fatal("want error")
	}
	_ = SetP2PMode("legacy")
	if GetP2PMode() != P2PModeLegacy {
		t.Fatalf("want legacy")
	}
}
