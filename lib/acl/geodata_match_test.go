package acl

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMatchGeoIPPublicAPI(t *testing.T) {
	resetGeoTestData(t)
	path := filepath.Join(t.TempDir(), "geoip.dat")
	writeGeoTestFile(t, path, "cn,203.0.113.0/24\n", time.Time{})
	SetGeoData(path, "")

	if !MatchGeoIP("cn", "203.0.113.10") {
		t.Fatal("期望命中 cn")
	}
	if MatchGeoIP("cn", "198.51.100.1") {
		t.Fatal("不应命中国外地址")
	}
	if !MatchGeoIP("private", "10.1.2.3") {
		t.Fatal("private 应命中内网")
	}
	if !MatchGeoIP("!cn", "198.51.100.1") {
		t.Fatal("!cn 应对非 cn 为真")
	}
	if !HasGeoIPCategory("cn") || !HasGeoIPCategory("private") {
		t.Fatal("类别应可用")
	}
}
