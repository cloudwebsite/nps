package waf

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ehang.io/nps/lib/acl"
)

func TestWAFDisabledBypass(t *testing.T) {
	Configure(Options{Enable: false})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/wp-login.php", nil)
	req.Header.Set("User-Agent", "sqlmap")
	res := CheckHTTP(req, "8.8.8.8", false)
	if !res.Allowed {
		t.Fatal("waf_enable=false 时应旁路")
	}
}

func TestUABlacklistHTTPOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{
		"ua_blacklist": ["sqlmap"],
		"ua_blacklist_is_regex": false,
		"foreign_allow_codes": ["CN"],
		"rules": []
	}`)
	Configure(Options{
		Enable: true, UABlacklistEnable: true,
		RulesPath: path,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("User-Agent", "Mozilla sqlmap/1.0")
	if res := CheckHTTP(req, "1.2.3.4", false); res.Allowed || res.Reason != ReasonUA {
		t.Fatalf("期望 UA 拦截, got %+v", res)
	}
	if res := CheckIP("1.2.3.4"); !res.Allowed {
		t.Fatal("TCP CheckIP 不应受 UA 黑名单影响")
	}
}

func TestBlockForeignWithGeoText(t *testing.T) {
	acl.SetGeoData("", "")
	t.Cleanup(func() { acl.SetGeoData("", "") })

	geoPath := filepath.Join(t.TempDir(), "geoip.dat")
	if err := os.WriteFile(geoPath, []byte("cn,203.0.113.0/24\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	acl.SetGeoData(geoPath, "")

	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{"foreign_allow_codes":["CN"],"rules":[]}`)
	Configure(Options{
		Enable: true, BlockForeignEnable: true,
		BlockForeignUnknown: true, RulesPath: path,
	})

	if res := CheckIP("203.0.113.8"); !res.Allowed {
		t.Fatalf("国内 IP 应放行: %+v", res)
	}
	if res := CheckIP("198.51.100.8"); res.Allowed || res.Reason != ReasonForeign {
		t.Fatalf("国外 IP 应拦截: %+v", res)
	}
	if res := CheckIP("127.0.0.1"); !res.Allowed {
		t.Fatal("环回应放行")
	}
}

func TestBlockForeignSkipWhenGeoMissing(t *testing.T) {
	acl.SetGeoData("", "")
	t.Cleanup(func() { acl.SetGeoData("", "") })

	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{"foreign_allow_codes":["CN"]}`)
	Configure(Options{
		Enable: true, BlockForeignEnable: true,
		BlockForeignUnknown: true, RulesPath: path,
	})
	if res := CheckIP("8.8.8.8"); !res.Allowed {
		t.Fatalf("缺 geo 时应跳过禁国外: %+v", res)
	}
}

func TestCountryHeaderTrusted(t *testing.T) {
	acl.SetGeoData("", "")
	t.Cleanup(func() { acl.SetGeoData("", "") })

	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{"foreign_allow_codes":["CN"]}`)
	Configure(Options{
		Enable: true, BlockForeignEnable: true,
		CountryHeaders: []string{"CF-IPCountry"},
		RulesPath:      path,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("CF-IPCountry", "CN")
	if res := CheckHTTP(req, "8.8.8.8", true); !res.Allowed {
		t.Fatalf("可信 CN 头应放行: %+v", res)
	}
	if res := CheckHTTP(req, "8.8.8.8", false); res.Allowed {
		// 无 geo 时不可信头被忽略 → 跳过禁国外 → 仍放行
		// 再测国外明确头
	}
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req2.Header.Set("CF-IPCountry", "US")
	if res := CheckHTTP(req2, "8.8.8.8", true); res.Allowed || res.Reason != ReasonForeign {
		t.Fatalf("可信 US 头应拦截: %+v", res)
	}
}

func TestAllowRuleShortCircuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{
		"ua_blacklist": ["sqlmap"],
		"ip_allowlist": ["203.0.113.0/24"],
		"rules": []
	}`)
	Configure(Options{
		Enable: true, UABlacklistEnable: true, CustomRulesEnable: true,
		RulesPath: path,
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header.Set("User-Agent", "sqlmap")
	if res := CheckHTTP(req, "203.0.113.9", false); !res.Allowed {
		t.Fatalf("allowlist 应短路 UA: %+v", res)
	}
}

func TestCustomDenyPathHTTP(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{
		"rules": [{
			"id": "deny-wp",
			"enable": true,
			"action": "deny",
			"priority": 100,
			"when": {"path_prefix": ["/wp-login.php"]},
			"scope": "http"
		}]
	}`)
	Configure(Options{Enable: true, CustomRulesEnable: true, RulesPath: path})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/wp-login.php", nil)
	if res := CheckHTTP(req, "1.1.1.1", false); res.Allowed || res.RuleID != "deny-wp" {
		t.Fatalf("期望路径 deny: %+v", res)
	}
	if res := CheckIP("1.1.1.1"); !res.Allowed {
		t.Fatal("TCP 不应命中 http scope 规则")
	}
}

func TestHotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "waf.json")
	writeJSON(t, path, `{"ua_blacklist":["oldbot"],"ua_blacklist_is_regex":false}`)
	Configure(Options{Enable: true, UABlacklistEnable: true, RulesPath: path})
	time.Sleep(10 * time.Millisecond)
	writeJSON(t, path, `{"ua_blacklist":["newbot"],"ua_blacklist_is_regex":false}`)
	if err := reloadRules(true); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://x/", nil)
	req.Header.Set("User-Agent", "newbot/1")
	if res := CheckHTTP(req, "1.1.1.1", false); res.Allowed {
		t.Fatal("热加载后应拦截 newbot")
	}
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
