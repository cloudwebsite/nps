package proxy

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/notify"
	"ehang.io/nps/web"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

var (
	errorPageMu       sync.RWMutex
	errorPageTemplate []byte
)

const fallbackErrorText = "nps error"

// LoadErrorPageContent 启动时加载错误页（自定义路径优先，否则 embed）；失败回退纯文本。
func LoadErrorPageContent() []byte {
	custom := strings.TrimSpace(beego.AppConfig.String("http_error_page_path"))

	var (
		raw []byte
		err error
	)
	if custom != "" {
		path := custom
		if !filepath.IsAbs(path) {
			path = filepath.Join(common.GetRunPath(), path)
		}
		raw, err = os.ReadFile(path)
		if err != nil {
			logs.Warn("http_error_page_path 读取失败 %s: %v，回退内置页", path, err)
		}
	}
	if len(raw) == 0 {
		raw, err = web.ReadStaticFile("page/error.html")
		if err != nil || len(raw) == 0 {
			logs.Warn("内置错误页读取失败: %v，使用纯文本兜底", err)
			raw = []byte(fallbackErrorText)
		}
	}

	errorPageMu.Lock()
	errorPageTemplate = append([]byte(nil), raw...)
	errorPageMu.Unlock()
	return append([]byte(nil), raw...)
}

// RenderErrorPage 白名单占位符替换；对 host/message 做 HTML 转义。
func RenderErrorPage(tpl []byte, status, host, message string) []byte {
	errorPageMu.RLock()
	if len(tpl) == 0 {
		tpl = errorPageTemplate
	}
	errorPageMu.RUnlock()

	brand := file.EffectiveHttpErrorPageBrand()
	if brand == "" {
		brand = "nps"
	}
	brandURL := file.EffectiveHttpErrorPageBrandURL()

	if len(tpl) == 0 {
		tpl = []byte(fallbackErrorText)
	}
	if status == "" {
		status = "502"
	}
	if message == "" {
		message = "Cannot connect to the server"
	}
	now := time.Now().Format(time.RFC3339)
	hostEsc := html.EscapeString(host)
	msgEsc := html.EscapeString(message)
	brandEsc := html.EscapeString(brand)
	brandURLEsc := html.EscapeString(brandURL)
	brandLink := brandEsc
	if brandURLEsc != "" {
		brandLink = `<a href="` + brandURLEsc + `">` + brandEsc + `</a>`
	}

	s := string(tpl)
	repl := []struct{ old, new string }{
		{"{{status}}", status},
		{"{{host}}", hostEsc},
		{"{{message}}", msgEsc},
		{"{{time}}", now},
		{"{{brand}}", brandEsc},
		{"{{brand_url}}", brandURLEsc},
		{"{{brand_link}}", brandLink},
	}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return []byte(s)
}

// notifyErrorPageHit 异步通知错误页命中（限流在 notify 内）。
func notifyErrorPageHit(host, message, addr string) {
	if !notify.Enabled() {
		return
	}
	body := "host=" + host
	if message != "" {
		body += "\nmessage=" + message
	}
	if addr != "" {
		body += "\naddr=" + addr
	}
	notify.Emit(notify.Event{
		Type:    notify.TypeErrorPage,
		Host:    host,
		Addr:    addr,
		Body:    body,
		Details: map[string]string{"message": message},
	})
}
