package web

import (
	"crypto/subtle"
	"html"
	"net/http"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/waf"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/astaxie/beego/logs"
)

// RegisterWAFPanelFilter 在管理面板 HTTP 上挂 WAF（须在 InitBeforeHTTPRun 之后）。
// 受 waf_enable + waf_protect_panel 控制；规则与域名反代共用 waf.json。
func RegisterWAFPanelFilter() {
	beego.InsertFilter("*", beego.BeforeRouter, panelWAFFilter)
}

func panelWAFFilter(ctx *context.Context) {
	if ctx == nil || ctx.Request == nil {
		return
	}
	o := waf.CurrentOptions()
	if !o.Enable || !o.ProtectPanel {
		return
	}
	r := ctx.Request
	ip := panelEffectiveClientIP(r)
	if ip == "" {
		ip = r.RemoteAddr
	}
	res := waf.CheckHTTP(r, ip, panelTrustCountryHeader(r))
	if res.Allowed {
		return
	}
	logs.Warn("WAF 拦截面板 reason=%s ip=%s path=%s", res.Reason, common.GetIpByAddr(ip), r.URL.Path)
	ctx.Output.Header("Content-Type", "text/plain; charset=utf-8")
	ctx.Abort(http.StatusForbidden, "Forbidden")
}

func panelTrustCountryHeader(r *http.Request) bool {
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

func panelEffectiveClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	return common.EffectiveHTTPClientIP(r.RemoteAddr, r.Header, panelTrustCountryHeader(r))
}
