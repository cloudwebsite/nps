package waf

import "strings"

// 内置场景 ID（面板不可删对应规则；可关可改）。
const (
	PresetUAScanner = "preset_ua_scanner"
	PresetUASEO     = "preset_ua_seo"
	PresetPathWP    = "preset_path_wp"
)

// PresetInfo 面板展示用场景元数据。
type PresetInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Kind        string `json:"kind"` // ua_scanner | ua_seo | path_wp | hint_foreign
}

// BuiltinPresets 一键防护卡片列表（U3A：代码内置）。
func BuiltinPresets() []PresetInfo {
	return []PresetInfo{
		{
			ID:          PresetUAScanner,
			Title:       "拦扫描器 UA",
			Description: "常见漏洞扫描、webshell 管理端等工具的浏览器标识（关键词包含即拦）",
			Kind:        "ua_scanner",
		},
		{
			ID:          PresetUASEO,
			Title:       "拦 SEO / 采集机器人",
			Description: "常见 SEO 爬虫、脚本库、命令行下载工具 UA（可能误伤正规监控，请按需关闭）",
			Kind:        "ua_seo",
		},
		{
			ID:          PresetPathWP,
			Title:       "拦 WordPress 扫描路径",
			Description: "拦截 /wp-login.php、/xmlrpc.php 等常见探测路径（仅域名 HTTP）",
			Kind:        "path_wp",
		},
		{
			ID:          "preset_block_foreign",
			Title:       "只允许国内访问",
			Description: "需在「服务端配置」打开 waf_block_foreign_enable 并重启；境外运维请先加 IP 例外",
			Kind:        "hint_foreign",
		},
	}
}

// PresetUAScannerKeywords 扫描器 UA 推荐词（子串匹配）。
func PresetUAScannerKeywords() []ListedItem {
	words := []string{
		"sqlmap", "nikto", "nessus", "nmap", "masscan", "zgrab", "wpscan",
		"dirbuster", "gobuster", "ffuf", "nuclei", "httpx", "acunetix",
		"appscan", "netsparker", "burpsuite", "owasp", "w3af", "havij",
		"antsword", "chinachopper", "behinder", "godzilla", "harvest", "censys",
	}
	return keywordsAsItems(words, "扫描器", PresetUAScanner)
}

// PresetUASEOKeywords SEO/采集 UA 推荐词。
func PresetUASEOKeywords() []ListedItem {
	words := []string{
		"AhrefsBot", "SemrushBot", "MJ12bot", "DotBot", "PetalBot", "Bytespider",
		"DataForSeoBot", "BLEXBot", "Seekport", "SeznamBot", "YandexBot",
		"python-requests", "Scrapy", "HttpClient", "Go-http-client", "curl/", "wget/",
		"libwww-perl", "Java/", "okhttp", "Apache-HttpClient",
	}
	return keywordsAsItems(words, "SEO/采集", PresetUASEO)
}

func keywordsAsItems(words []string, remark, tag string) []ListedItem {
	out := make([]ListedItem, 0, len(words))
	for _, w := range words {
		out = append(out, ListedItem{
			Value:  w,
			Remark: remark + " · " + tag,
			Enable: boolPtr(true),
		})
	}
	return out
}

// PresetPathWPRule WordPress 路径拦截规则。
func PresetPathWPRule() CustomRule {
	return CustomRule{
		ID:       PresetPathWP,
		Enable:   true,
		Action:   "deny",
		Priority: 100,
		Scope:    ScopeHTTP,
		Remark:   "WordPress 扫描路径",
		Builtin:  true,
		When: RuleWhen{
			Methods:    []string{"GET", "POST", "HEAD"},
			PathPrefix: []string{"/wp-login.php", "/xmlrpc.php", "/wp-admin", "/wp-content/plugins"},
		},
	}
}

// ApplyPreset 将内置场景合并进配置；已存在同 value / 同 rule id 则跳过或补标记。
func ApplyPreset(cfg *FileConfig, presetID string) bool {
	if cfg == nil {
		return false
	}
	NormalizeFileConfig(cfg)
	switch presetID {
	case PresetUAScanner:
		cfg.UABlacklistIsRegex = false
		cfg.UABlacklistItems = mergeUAItems(cfg.UABlacklistItems, PresetUAScannerKeywords())
	case PresetUASEO:
		cfg.UABlacklistIsRegex = false
		cfg.UABlacklistItems = mergeUAItems(cfg.UABlacklistItems, PresetUASEOKeywords())
	case PresetPathWP:
		cfg.Rules = upsertBuiltinRule(cfg.Rules, PresetPathWPRule())
	default:
		return false
	}
	NormalizeFileConfig(cfg)
	return true
}

func mergeUAItems(existing, add []ListedItem) []ListedItem {
	seen := map[string]struct{}{}
	out := make([]ListedItem, 0, len(existing)+len(add))
	for _, it := range existing {
		v := strings.TrimSpace(it.Value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		seen[key] = struct{}{}
		out = append(out, it)
	}
	for _, it := range add {
		v := strings.TrimSpace(it.Value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, it)
	}
	return out
}

func upsertBuiltinRule(rules []CustomRule, neu CustomRule) []CustomRule {
	for i := range rules {
		if rules[i].ID == neu.ID {
			rules[i].Builtin = true
			if rules[i].Remark == "" {
				rules[i].Remark = neu.Remark
			}
			if len(rules[i].When.PathPrefix) == 0 {
				rules[i].When = neu.When
			}
			if rules[i].Action == "" {
				rules[i].Action = neu.Action
			}
			if rules[i].Scope == "" {
				rules[i].Scope = neu.Scope
			}
			return rules
		}
	}
	return append(rules, neu)
}
