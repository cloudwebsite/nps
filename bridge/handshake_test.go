package bridge

import (
	"testing"

	"ehang.io/nps/lib/version"
)

func TestAuthenticatePathSelection(t *testing.T) {
	version.SetSecureHandshakeServerEnabled(false)
	if version.ServerSelectsSecureHandshake("0.26.41.1") {
		t.Fatal("switch off")
	}
	version.SetSecureHandshakeServerEnabled(true)
	defer version.SetSecureHandshakeServerEnabled(false)
	if !version.ServerSelectsSecureHandshake("0.26.41") {
		t.Fatal("switch on + new client")
	}
	if version.ServerSelectsSecureHandshake("0.26.40") {
		t.Fatal("old client")
	}
}
