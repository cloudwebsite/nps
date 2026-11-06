package proxy

import (
	"crypto/subtle"
	"html"
	"net/http"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/waf"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// trustHTTPCountryHeader 与 effectiveHTTPRemoteAddr 对齐：仅在信任转发头时读国家头。
func trustHTTPCountryHeader(r *http.Request) bool {
	if r == nil {
		return false
	}
	allowForwarded := file.EffectiveAllowXRealIp()
	if !allowForwarded {
		secret := strings.TrimSpace(beego.AppConfig.String("x_nps_http_only"))
		provided := strings.TrimSpace(r.Header.Get("X-NPS-Http-Only"))
		allowForwarded = secret != "" && provided != "" &&
			subtle.ConstantTimeCompare([]byte(html.UnescapeString(secret)), []byte(provided)) == 1
	}
	return allowForwarded
}

// checkHTTPWAF 域名 HTTP 入口 WAF；拦截时写 403。返回 true 表示已拦截。
func checkHTTPWAF(w http.ResponseWriter, r *http.Request) bool {
	if !waf.Enabled() || r == nil {
		return false
	}
	ip := effectiveHTTPRemoteAddr(r)
	if ip == "" {
		ip = r.RemoteAddr
	}
	res := waf.CheckHTTP(r, ip, trustHTTPCountryHeader(r))
	if res.Allowed {
		return false
	}
	logs.Warn("WAF 拦截 HTTP reason=%s ip=%s host=%s path=%s", res.Reason, common.GetIpByAddr(ip), r.Host, r.URL.Path)
	if w != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}
	return true
}

// checkTCPWAF TCP 等入口；拦截返回 true（调用方应关闭连接）。
func checkTCPWAF(remoteAddr string) bool {
	if !waf.Enabled() {
		return false
	}
	res := waf.CheckIP(remoteAddr)
	if res.Allowed {
		return false
	}
	logs.Warn("WAF 拦截 TCP reason=%s ip=%s", res.Reason, common.GetIpByAddr(remoteAddr))
	return true
}
