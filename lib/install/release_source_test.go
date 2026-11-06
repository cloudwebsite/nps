package install

import "testing"

func TestSetReleaseSource(t *testing.T) {
	defer ResetReleaseSource()
	SetReleaseSource("https://mirror.example/api/latest", "https://mirror.example/dl")
	api, base := ReleaseSourceSnapshot()
	if api != "https://mirror.example/api/latest" || base != "https://mirror.example/dl" {
		t.Fatalf("got %q %q", api, base)
	}
	ResetReleaseSource()
	api, base = ReleaseSourceSnapshot()
	if api != defaultReleaseAPIURL || base != defaultReleaseDownloadBase {
		t.Fatalf("reset failed: %q %q", api, base)
	}
}
