package controllers

import (
	"net/http"

	"ehang.io/nps/web"
	"github.com/astaxie/beego"
)

// SecureEntryController 处理安全入口解锁路径。
type SecureEntryController struct {
	beego.Controller
}

// Unlock 写入 Session 解锁标记并跳转登录页。
func (s *SecureEntryController) Unlock() {
	s.SetSession(web.SessionKeyWebSecureOK, true)
	s.Redirect(web.WebSecureLoginPath(), http.StatusFound)
}

// NormalizeWebSecureEntry 转发到 web 包校验（供服务端配置页复用）。
func NormalizeWebSecureEntry(raw string) (string, error) {
	return web.NormalizeWebSecureEntry(raw)
}

// RegisterSecureEntryRoutes 注册解锁路径（配置为空时跳过）。
func RegisterSecureEntryRoutes() {
	unlock := web.WebSecureUnlockPath()
	if unlock == "" {
		return
	}
	beego.Router(unlock, &SecureEntryController{}, "*:Unlock")
}
