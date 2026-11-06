package acl

import (
	"net"
	"strings"
)

// MatchIP 判断 ip 是否命中规则列表中的任一条。
// 规则可为精确 IP、CIDR、geoip:类别（含 !）；空行与 #/; 注释忽略。
// geosite: 在无 host 时不命中（请用 MatchRules）。
// 空列表 → false（由调用方决定「空=不限制」还是「空=拒绝」）。
func MatchIP(ip string, rules []string) bool {
	return MatchRules(ip, "", rules)
}

// HasActiveRules 列表是否含有效规则（非空、非纯注释）。
func HasActiveRules(rules []string) bool {
	return len(expandRules(rules)) > 0
}

func expandRules(rules []string) []string {
	var out []string
	for _, raw := range rules {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\r' || r == '\n' || r == ','
		}) {
			rule := strings.TrimSpace(part)
			if rule == "" || strings.HasPrefix(rule, "#") || strings.HasPrefix(rule, ";") {
				continue
			}
			out = append(out, rule)
		}
	}
	return out
}

// ParseIPFromAddr 从 host:port / 纯 IP 提取 IP 字符串。
func ParseIPFromAddr(ipPort string) string {
	ipPort = strings.TrimSpace(ipPort)
	if host, _, err := net.SplitHostPort(ipPort); err == nil {
		return host
	}
	if strings.HasPrefix(ipPort, "[") && strings.HasSuffix(ipPort, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(ipPort, "["), "]")
	}
	if net.ParseIP(ipPort) != nil {
		return ipPort
	}
	if i := strings.LastIndexByte(ipPort, ':'); i > 0 {
		if net.ParseIP(ipPort[:i]) != nil {
			return ipPort[:i]
		}
	}
	return ipPort
}
