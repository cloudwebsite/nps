package bridge

import (
	"errors"
	"testing"

	"ehang.io/nps/lib/version"
)

func TestClientSupportsRemoteUpdateGate(t *testing.T) {
	cases := []struct {
		ver  string
		want bool
	}{
		{"", false},
		{"0.26.40", false},
		{"0.26.41", true},
		{"0.26.41.1", true},
		{"v0.26.41", true},
		{"not-a-version", false},
	}
	for _, tc := range cases {
		if got := clientSupportsRemoteUpdate(tc.ver); got != tc.want {
			t.Fatalf("ver=%q got=%v want=%v (floor=%s)", tc.ver, got, tc.want, version.RemoteUpdateMinVersion)
		}
	}
}

func TestErrUpdateUnsupportedMentionsFloor(t *testing.T) {
	if !errors.Is(ErrUpdateUnsupported, ErrUpdateUnsupported) {
		t.Fatal("sentinel")
	}
	if ErrUpdateUnsupported == nil || len(ErrUpdateUnsupported.Error()) < 8 {
		t.Fatal("error text")
	}
}
