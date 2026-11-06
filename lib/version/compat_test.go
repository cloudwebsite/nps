package version

import "testing"

func TestCoreVersionLocked(t *testing.T) {
	if GetVersion() != "0.26.0" || CoreVersion != "0.26.0" {
		t.Fatalf("core=%q GetVersion=%q", CoreVersion, GetVersion())
	}
}

func TestLegacyHandshakeDefaultServerOff(t *testing.T) {
	SetSecureHandshakeServerEnabled(false)
	defer SetSecureHandshakeServerEnabled(false)

	if !SupportsLegacyHandshake("0.26.40") {
		t.Fatal("0.26.40 must remain legacy")
	}
	if UsesSecureHandshake("0.26.40") {
		t.Fatal("0.26.40 must not use secure")
	}
	if !UsesSecureHandshake("0.26.41.1") {
		t.Fatal("0.26.41.1 is capable of secure")
	}
	if ServerSelectsSecureHandshake("0.26.41.1") {
		t.Fatal("server must keep legacy while switch off")
	}
}

func TestServerSelectsSecureWhenEnabled(t *testing.T) {
	SetSecureHandshakeServerEnabled(true)
	defer SetSecureHandshakeServerEnabled(false)

	if !ServerSelectsSecureHandshake("0.26.41") {
		t.Fatal("enabled + 0.26.41 should select secure")
	}
	if ServerSelectsSecureHandshake("0.26.40") {
		t.Fatal("old clients must stay legacy even when switch on")
	}
}

func TestCompareDotVersion(t *testing.T) {
	cmp, err := CompareDotVersion("0.26.41.1", "0.26.0")
	if err != nil || cmp <= 0 {
		t.Fatalf("cmp=%d err=%v", cmp, err)
	}
	cmp, err = CompareDotVersion("0.26.41.1", "0.26.41")
	if err != nil || cmp <= 0 {
		t.Fatalf("build segment cmp=%d err=%v", cmp, err)
	}
}

func TestSupportsRemoteUpdate(t *testing.T) {
	if SupportsRemoteUpdate("") || SupportsRemoteUpdate("bad") || SupportsRemoteUpdate("0.26.40") {
		t.Fatal("old/empty/unparseable must be gated off")
	}
	if !SupportsRemoteUpdate("0.26.41") || !SupportsRemoteUpdate("0.26.41.1") {
		t.Fatal("remote-update floor and newer must pass")
	}
}
