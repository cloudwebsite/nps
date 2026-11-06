package acl

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNormalizePriority(t *testing.T) {
	if NormalizePriority(AllowFirst) != AllowFirst {
		t.Fatal("AllowFirst")
	}
	if NormalizePriority(99) != DenyFirst {
		t.Fatal("未知应回落 DenyFirst")
	}
}

func TestNormalizeRules(t *testing.T) {
	out, err := NormalizeRules([]string{
		"10.0.0.0/8",
		"geoip:CN",
		"geoip:!private",
		"geosite:Google",
		"not-a-rule",
		"10.0.0.0/8",
		"# comment",
	})
	if len(out) != 4 {
		t.Fatalf("期望 4 条有效，得 %v", out)
	}
	if out[0] != "10.0.0.0/8" || out[1] != "geoip:cn" || out[2] != "geoip:!private" || out[3] != "geosite:google" {
		t.Fatalf("规范化结果异常: %v", out)
	}
	if err == nil {
		t.Fatal("应报告无效规则")
	}
}

func TestAllowsDenyFirstWhiteOnly(t *testing.T) {
	p := SourcePolicy{Priority: DenyFirst, White: []string{"10.0.0.1"}}
	if !Allows("10.0.0.1", "", p) {
		t.Fatal("白名单内应放行")
	}
	if Allows("10.0.0.2", "", p) {
		t.Fatal("白名单外 DenyFirst 应拒绝")
	}
}

func TestAllowsDenyFirstBlackOnly(t *testing.T) {
	p := SourcePolicy{Priority: DenyFirst, Black: []string{"10.0.0.1"}}
	if Allows("10.0.0.1", "", p) {
		t.Fatal("黑名单应拒绝")
	}
	if !Allows("10.0.0.2", "", p) {
		t.Fatal("未命中黑名单应放行")
	}
}

func TestAllowsAllowFirst(t *testing.T) {
	p := SourcePolicy{
		Priority: AllowFirst,
		Black:    []string{"10.0.0.0/8"},
		White:    []string{"10.0.0.5"},
	}
	if !Allows("10.0.0.5", "", p) {
		t.Fatal("白优先应放行")
	}
	if Allows("10.0.0.9", "", p) {
		t.Fatal("仅黑命中应拒绝")
	}
	if !Allows("1.2.3.4", "", p) {
		t.Fatal("两边未命中 AllowFirst 应放行")
	}
}

func TestAllowsLevels(t *testing.T) {
	g := SourcePolicy{Black: []string{"1.1.1.1"}}
	c := SourcePolicy{White: []string{"2.2.2.2"}, Priority: DenyFirst}
	if AllowsLevels("1.1.1.1", "", g, c) {
		t.Fatal("全局黑应挡住")
	}
	if !AllowsLevels("2.2.2.2", "", SourcePolicy{}, c) {
		t.Fatal("仅客户端白应放行")
	}
	if AllowsLevels("3.3.3.3", "", SourcePolicy{}, c) {
		t.Fatal("客户端 DenyFirst 白名单外应拒绝")
	}
}

func TestMatchRuleGeoIP(t *testing.T) {
	resetGeoTestData(t)
	path := filepath.Join(t.TempDir(), "geoip.dat")
	writeGeoTestFile(t, path, "cn,203.0.113.0/24\n", time.Time{})
	SetGeoData(path, "")

	if !MatchRule("203.0.113.10", "", "geoip:cn") {
		t.Fatal("geoip:cn 应命中")
	}
	if !MatchIP("203.0.113.10", []string{"geoip:cn"}) {
		t.Fatal("MatchIP 应支持 geoip:")
	}
	if MatchRule("198.51.100.1", "", "geoip:cn") {
		t.Fatal("非 cn 不应命中")
	}
}
