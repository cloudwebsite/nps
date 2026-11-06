package proxy

import (
	"crypto/subtle"
	"html"
	"net/http"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
)

// effectiveHTTPRemoteAddr 返回 HTTP ACL / WAF / 日志使用的访客地址。
// allow_x_real_ip 默认 true；关闭后仅当请求携带与 x_nps_http_only 匹配的密钥头时才信转发头。
// 信头时按内置 CDN/反代头顺序探测（见 common.DefaultHTTPRealIPHeaders）。
func effectiveHTTPRemoteAddr(r *http.Request) string {
	if r == nil {
		return ""
	}
	allowForwarded := file.EffectiveAllowXRealIp()
	if !allowForwarded {
		secret := strings.TrimSpace(beego.AppConfig.String("x_nps_http_only"))
		provided := strings.TrimSpace(r.Header.Get("X-NPS-Http-Only"))
		allowForwarded = secret != "" && provided != "" &&
			subtle.ConstantTimeCompare([]byte(html.UnescapeString(secret)), []byte(provided)) == 1
	}
	return common.EffectiveHTTPClientIP(r.RemoteAddr, r.Header, allowForwarded)
}
