package tool

import (
	"testing"

	"ehang.io/nps/lib/file"
)

func TestEffectiveClientTunnelLimit(t *testing.T) {
	if EffectiveClientTunnelLimit(nil) != MaxTunnelsPerClient() {
		t.Fatal("nil client")
	}
	c := &file.Client{MaxTunnelNum: 3}
	if EffectiveClientTunnelLimit(c) != 3 {
		t.Fatal("client override")
	}
}
