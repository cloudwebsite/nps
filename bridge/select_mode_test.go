package bridge

import "testing"

func TestSetClientSelectMode(t *testing.T) {
	if err := SetClientSelectMode("roundrobin"); err != nil {
		t.Fatal(err)
	}
	if ClientSelectMode() != RoundRobin || ClientSelectModeName() != "roundrobin" {
		t.Fatalf("got %v %s", ClientSelectMode(), ClientSelectModeName())
	}
	if err := SetClientSelectMode("bad"); err == nil {
		t.Fatal("expected error")
	}
	if ClientSelectMode() != Primary {
		t.Fatal("invalid mode should reset to primary")
	}
	_ = SetClientSelectMode("primary")
}

func TestParseRequireTLS(t *testing.T) {
	if ParseRequireTLS("true") != true {
		t.Fatal("true")
	}
	if ParseRequireTLS("") || ParseRequireTLS("auto") || ParseRequireTLS("false") {
		t.Fatal("default/auto/false must keep plaintext for upgrade")
	}
}
