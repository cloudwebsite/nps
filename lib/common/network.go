package common

import (
	"net"
	"net/http"
	"strings"
)

// DefaultHTTPRealIPHeaders 域名 HTTP 取访客 IP 时的内置头顺序（不可配置）。
// 优先 CDN 单值头，再 X-Real-IP / X-Forwarded-For，最后其它别名与 RFC Forwarded。
var DefaultHTTPRealIPHeaders = []string{
	"CF-Connecting-IP",
	"True-Client-IP",
	"EO-Connecting-IP",
	"Ali-Cdn-Real-IP",
	"Cdn-Src-IP",
	"Cdn-Real-IP",
	"X-Real-IP",
	"X-Forwarded-For",
	"X-Forwarded",
	"Forwarded-For",
	"Forwarded",
	"Client-IP",
	"X-Cluster-Client-IP",
	"WL-Proxy-Client-IP",
	"Proxy-Client-IP",
}

// EffectiveHTTPClientIP 返回 HTTP 反代 ACL / WAF / 日志应使用的访客地址。
// 直连对端始终为回退；allowForwarded=true 时按 DefaultHTTPRealIPHeaders 取第一个合法 IP。
// 配合 allow_x_real_ip（默认开）；不校验 trusted_proxy_ips。
func EffectiveHTTPClientIP(remoteAddr string, header http.Header, allowForwarded bool) string {
	peer := GetIpByAddr(strings.TrimSpace(remoteAddr))
	if peer == "" {
		peer = strings.TrimSpace(remoteAddr)
	}
	if !allowForwarded || header == nil {
		return peer
	}
	for _, name := range DefaultHTTPRealIPHeaders {
		raw := strings.TrimSpace(header.Get(name))
		if raw == "" {
			continue
		}
		var candidate string
		if strings.EqualFold(name, "Forwarded") {
			candidate = firstForwardedForIP(raw)
		} else {
			candidate = firstForwardedValue(raw)
		}
		if ip := parseClientIPCandidate(candidate); ip != "" {
			return ip
		}
	}
	return peer
}

func firstForwardedValue(value string) string {
	if index := strings.IndexByte(value, ','); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return strings.TrimSpace(value)
}

// firstForwardedForIP 从 RFC 7239 Forwarded 头取第一个 for= 中的 IP。
func firstForwardedForIP(value string) string {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, kv := range strings.Split(part, ";") {
			kv = strings.TrimSpace(kv)
			eq := strings.IndexByte(kv, '=')
			if eq <= 0 {
				continue
			}
			key := strings.TrimSpace(kv[:eq])
			if !strings.EqualFold(key, "for") {
				continue
			}
			return strings.TrimSpace(kv[eq+1:])
		}
	}
	return ""
}

func parseClientIPCandidate(candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || strings.EqualFold(candidate, "unknown") {
		return ""
	}
	candidate = strings.Trim(candidate, `"`)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	// [IPv6]:port 或 [IPv6]
	if strings.HasPrefix(candidate, "[") {
		if end := strings.IndexByte(candidate, ']'); end > 1 {
			inner := candidate[1:end]
			if ip := net.ParseIP(inner); ip != nil {
				return ip.String()
			}
			return ""
		}
	}
	// host:port（IPv4）
	if host, _, err := net.SplitHostPort(candidate); err == nil {
		candidate = host
	}
	candidate = strings.Trim(candidate, "[]")
	if ip := net.ParseIP(candidate); ip != nil {
		return ip.String()
	}
	return ""
}
