package waf

import "strings"

// NormalizeFileConfig 同步 items ↔ 扁平列表，供面板与引擎共用。
// 优先以 *_items 为准生成扁平数组；仅有扁平数组时回填 items（无说明、默认启用）。
func NormalizeFileConfig(cfg *FileConfig) {
	if cfg == nil {
		return
	}
	cfg.UABlacklistItems = normalizeItems(cfg.UABlacklistItems, cfg.UABlacklist)
	cfg.UABlacklist = flatEnabledValues(cfg.UABlacklistItems)

	cfg.IPAllowlistItems = normalizeItems(cfg.IPAllowlistItems, cfg.IPAllowlist)
	cfg.IPAllowlist = flatEnabledValues(cfg.IPAllowlistItems)

	for i := range cfg.Rules {
		cfg.Rules[i].ID = strings.TrimSpace(cfg.Rules[i].ID)
		cfg.Rules[i].Action = strings.ToLower(strings.TrimSpace(cfg.Rules[i].Action))
		cfg.Rules[i].Scope = strings.ToLower(strings.TrimSpace(cfg.Rules[i].Scope))
		cfg.Rules[i].Remark = strings.TrimSpace(cfg.Rules[i].Remark)
	}
}

func normalizeItems(items []ListedItem, flat []string) []ListedItem {
	if len(items) == 0 {
		return itemsFromFlat(flat)
	}
	out := make([]ListedItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, it := range items {
		v := strings.TrimSpace(it.Value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		en := true
		if it.Enable != nil {
			en = *it.Enable
		}
		out = append(out, ListedItem{
			Value:  v,
			Remark: strings.TrimSpace(it.Remark),
			Enable: boolPtr(en),
		})
	}
	return out
}

func itemsFromFlat(flat []string) []ListedItem {
	out := make([]ListedItem, 0, len(flat))
	seen := map[string]struct{}{}
	for _, v := range flat {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ListedItem{Value: v, Enable: boolPtr(true)})
	}
	return out
}

func flatEnabledValues(items []ListedItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !it.IsEnabled() {
			continue
		}
		v := strings.TrimSpace(it.Value)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func effectiveUABlacklist(cfg FileConfig) []string {
	if len(cfg.UABlacklistItems) > 0 {
		return flatEnabledValues(cfg.UABlacklistItems)
	}
	return cfg.UABlacklist
}

func boolPtr(v bool) *bool { return &v }

// IsEnabled 缺省 enable 视为开启（兼容只写 value 的手写 JSON）。
func (it ListedItem) IsEnabled() bool {
	if it.Enable == nil {
		return true
	}
	return *it.Enable
}
