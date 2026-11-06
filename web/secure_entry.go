package web

import (
	"net/http"
	"path"
	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
)

// SessionKeyWebSecureOK 安全入口解锁 Session 键（与 Login/Unlock 共用）。
const SessionKeyWebSecureOK = "web_secure_ok"

const webSecureEntryMaxLen = 64

// NormalizeWebSecureEntry 规范化并校验安全入口路径；空表示关闭。
// 合法：以 / 开头、非根路径、不含 ..、长度 ≤ 64。
func NormalizeWebSecureEntry(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.Contains(raw, "..") {
		return "", errWebSecureEntry("不能包含 ..")
	}
	if !strings.HasPrefix(raw, "/") {
		return "", errWebSecureEntry("必须以 / 开头")
	}
	cleaned := path.Clean(raw)
	if cleaned == "/" || cleaned == "." {
		return "", errWebSecureEntry("不能仅为 /")
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if strings.Contains(cleaned, "..") {
		return "", errWebSecureEntry("不能包含 ..")
	}
	if len(cleaned) > webSecureEntryMaxLen {
		return "", errWebSecureEntry("长度不能超过 64")
	}
	return cleaned, nil
}

type webSecureEntryError string

func (e webSecureEntryError) Error() string { return string(e) }

func errWebSecureEntry(msg string) error {
	return webSecureEntryError("web_secure_entry " + msg)
}

func configuredWebSecureEntry() string {
	entry, err := NormalizeWebSecureEntry(beego.AppConfig.String("web_secure_entry"))
	if err != nil {
		return ""
	}
	return entry
}

func webBasePrefix() string {
	base := strings.TrimSuffix(strings.TrimSpace(beego.AppConfig.String("web_base_url")), "/")
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return path.Clean(base)
}

// WebSecureUnlockPath 返回含 web_base_url 前缀的完整解锁路径；关闭时为空。
func WebSecureUnlockPath() string {
	entry := configuredWebSecureEntry()
	if entry == "" {
		return ""
	}
	return path.Clean(webBasePrefix() + entry)
}

// WebSecureLoginPath 返回登录页路径。
func WebSecureLoginPath() string {
	return path.Clean(webBasePrefix() + "/login/index")
}

func normalizeRequestPath(p string) string {
	if p == "" {
		return "/"
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func webSecureExempt(reqPath string) bool {
	base := webBasePrefix()
	apiPrefix := path.Clean(base + "/api/v1")
	if reqPath == apiPrefix || strings.HasPrefix(reqPath, apiPrefix+"/") {
		return true
	}
	return false
}

func webSecureSessionOK(ctx *context.Context) bool {
	if ctx == nil || ctx.Input == nil || ctx.Input.CruSession == nil {
		return false
	}
	v := ctx.Input.CruSession.Get(SessionKeyWebSecureOK)
	ok, _ := v.(bool)
	return ok
}

// RegisterSecureEntryFilter 注册安全入口门闸。须在 beego.InitBeforeHTTPRun 之后调用，
// 以便 Session 过滤器先启动 CruSession。未解锁时面板与静态资源一律 404；/api/v1 放行。
func RegisterSecureEntryFilter() {
	beego.InsertFilter("*", beego.BeforeRouter, secureEntryFilter)
}

func secureEntryFilter(ctx *context.Context) {
	entry := configuredWebSecureEntry()
	if entry == "" {
		return
	}
	reqPath := normalizeRequestPath(ctx.Request.URL.Path)
	unlock := WebSecureUnlockPath()
	if unlock != "" && reqPath == unlock {
		return
	}
	if webSecureExempt(reqPath) {
		return
	}
	if webSecureSessionOK(ctx) {
		return
	}
	ctx.Output.Header("Content-Type", "text/plain; charset=utf-8")
	ctx.Abort(http.StatusNotFound, "404 page not found")
}
