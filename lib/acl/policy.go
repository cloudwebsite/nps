package acl

import (
	"fmt"
	"net/netip"
	"strings"
)

// 单层黑白名单优先级（与 djylb / geomatch 对齐）。
const (
	// DenyFirst：命中黑名单则拒绝；两边都未命中也拒绝（有白名单时须命中白名单）。
	DenyFirst = 0
	// AllowFirst：命中白名单则放行；两边都未命中也放行。
	AllowFirst = 1
)

// NormalizePriority 未知值回落到 DenyFirst。
func NormalizePriority(p int) int {
	if p == AllowFirst {
		return AllowFirst
	}
	return DenyFirst
}

// SourcePolicy 一层源地址策略（全局 / 客户端 / 隧道·域名）。
// 两列表皆空 → 本层不限制。
type SourcePolicy struct {
	Priority int
	Black    []string
	White    []string
}

// Empty 是否无有效规则。
func (p SourcePolicy) Empty() bool {
	return !HasActiveRules(p.Black) && !HasActiveRules(p.White)
}

// MatchRule 判断单条规则是否命中。
// 支持：精确 IP、CIDR、geoip:类别（含 ! 取反）、geosite:类别（需 host；无 host 则不命中）。
func MatchRule(ip, host, rule string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return false
	}
	lower := strings.ToLower(rule)
	switch {
	case strings.HasPrefix(lower, "geoip:"):
		cat := strings.TrimSpace(rule[len("geoip:"):])
		if cat == "" {
			return false
		}
		return MatchGeoIP(cat, ip)
	case strings.HasPrefix(lower, "geosite:"):
		cat := strings.TrimSpace(rule[len("geosite:"):])
		if cat == "" || strings.TrimSpace(host) == "" {
			return false
		}
		neg := strings.HasPrefix(cat, "!")
		if neg {
			cat = strings.TrimPrefix(cat, "!")
		}
		hit := MatchGeoSite(cat, host)
		return hit != neg
	default:
		return matchPlainIP(ip, rule)
	}
}

func matchPlainIP(ip, rule string) bool {
	ip = strings.TrimSpace(ip)
	rule = strings.TrimSpace(rule)
	if ip == "" || rule == "" {
		return false
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return rule == ip
	}
	addr = addr.Unmap()
	if prefix, err := netip.ParsePrefix(rule); err == nil {
		return prefix.Contains(addr)
	}
	if other, err := netip.ParseAddr(rule); err == nil {
		return other.Unmap() == addr
	}
	return rule == ip
}

// MatchRules 任一条命中即为 true；空列表 → false。
func MatchRules(ip, host string, rules []string) bool {
	for _, raw := range expandRules(rules) {
		if MatchRule(ip, host, raw) {
			return true
		}
	}
	return false
}

// Allows 单层策略：空策略放行；否则按优先级裁决。
func Allows(ip, host string, p SourcePolicy) bool {
	if p.Empty() {
		return true
	}
	inBlack := MatchRules(ip, host, p.Black)
	inWhite := MatchRules(ip, host, p.White)
	hasWhite := HasActiveRules(p.White)
	hasBlack := HasActiveRules(p.Black)
	switch NormalizePriority(p.Priority) {
	case AllowFirst:
		if inWhite {
			return true
		}
		if inBlack {
			return false
		}
		// 两边都未命中：放行
		return true
	default: // DenyFirst
		if inBlack {
			return false
		}
		if inWhite {
			return true
		}
		// 有白名单但未命中 → 拒绝；仅有黑名单且未命中 → 放行
		if hasWhite {
			return false
		}
		if hasBlack {
			return true
		}
		return true
	}
}

// AllowsLevels 三层均须放行；nil/空层跳过。
func AllowsLevels(ip, host string, levels ...SourcePolicy) bool {
	for _, lv := range levels {
		if !Allows(ip, host, lv) {
			return false
		}
	}
	return true
}

// NormalizeRules 规范化规则：去空/注释、小写 geo 前缀、去重；非法规则丢弃并记入 error。
// 返回规范化后的列表；err 汇总被丢弃项（可非空同时仍有有效 out）。
func NormalizeRules(rules []string) (out []string, err error) {
	seen := make(map[string]struct{})
	var rejected []string
	for _, raw := range expandRules(rules) {
		canon, ok := canonicalizeRule(raw)
		if !ok {
			rejected = append(rejected, raw)
			continue
		}
		if _, dup := seen[canon]; dup {
			continue
		}
		seen[canon] = struct{}{}
		out = append(out, canon)
	}
	if len(rejected) > 0 {
		err = fmt.Errorf("丢弃无效规则: %s", strings.Join(rejected, "; "))
	}
	return out, err
}

func canonicalizeRule(rule string) (string, bool) {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return "", false
	}
	lower := strings.ToLower(rule)
	switch {
	case strings.HasPrefix(lower, "geoip:"):
		cat := strings.TrimSpace(rule[len("geoip:"):])
		if cat == "" {
			return "", false
		}
		neg := strings.HasPrefix(cat, "!")
		if neg {
			cat = strings.TrimPrefix(cat, "!")
		}
		cat = strings.ToLower(strings.TrimSpace(cat))
		if cat == "" {
			return "", false
		}
		// private 内置；其余类别未加载数据时仍保留规则（运行时未命中）
		if neg {
			return "geoip:!" + cat, true
		}
		return "geoip:" + cat, true
	case strings.HasPrefix(lower, "geosite:"):
		cat := strings.TrimSpace(rule[len("geosite:"):])
		if cat == "" {
			return "", false
		}
		neg := strings.HasPrefix(cat, "!")
		if neg {
			cat = strings.TrimPrefix(cat, "!")
		}
		cat = strings.ToLower(strings.TrimSpace(cat))
		if cat == "" {
			return "", false
		}
		if neg {
			return "geosite:!" + cat, true
		}
		return "geosite:" + cat, true
	default:
		if _, err := netip.ParsePrefix(rule); err == nil {
			return rule, true
		}
		if addr, err := netip.ParseAddr(rule); err == nil {
			return addr.Unmap().String(), true
		}
		return "", false
	}
}
