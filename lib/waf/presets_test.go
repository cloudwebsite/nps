package waf

import "testing"

func TestApplyPresetUAAndPath(t *testing.T) {
	cfg := FileConfig{}
	if !ApplyPreset(&cfg, PresetUAScanner) {
		t.Fatal("apply scanner")
	}
	if len(cfg.UABlacklist) == 0 || cfg.UABlacklistIsRegex {
		t.Fatalf("期望子串 UA 列表, got regex=%v n=%d", cfg.UABlacklistIsRegex, len(cfg.UABlacklist))
	}
	if !ApplyPreset(&cfg, PresetPathWP) {
		t.Fatal("apply wp")
	}
	found := false
	for _, r := range cfg.Rules {
		if r.ID == PresetPathWP && r.Builtin {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("缺少 builtin WP 规则")
	}
	n := len(cfg.UABlacklist)
	_ = ApplyPreset(&cfg, PresetUAScanner)
	if len(cfg.UABlacklist) != n {
		t.Fatal("重复应用不应复制关键词")
	}
}
