package file

import (
	"strings"
	"unicode/utf8"

	"github.com/astaxie/beego"
)

const maxPanelTextRunes = 64

// NormalizePanelText 面板短文案（欢迎语/副标题等）；空串合法。
func NormalizePanelText(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	if utf8.RuneCountInString(name) > maxPanelTextRunes {
		runes := []rune(name)
		name = string(runes[:maxPanelTextRunes])
	}
	return name
}

func intPtr(v int) *int { return &v }

func intPtrValue(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func boolPtrOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func stringPtrOr(v *string, def string) string {
	if v == nil {
		return def
	}
	return *v
}

func appConfigInt(key string, def int) int {
	return beego.AppConfig.DefaultInt(key, def)
}

func appConfigString(key string) string {
	return beego.AppConfig.String(key)
}

// EffectiveWebWelcome 顶栏环境标识（可空）。
func EffectiveWebWelcome() string {
	if g := GetDb().GetGlobal(); g != nil && g.WebWelcome != nil {
		return NormalizePanelText(*g.WebWelcome)
	}
	return NormalizePanelText(appConfigString("web_welcome"))
}

// EffectiveWebSubtitle 侧栏副标题（可空=模板默认文案）。
func EffectiveWebSubtitle() string {
	if g := GetDb().GetGlobal(); g != nil && g.WebSubtitle != nil {
		return NormalizePanelText(*g.WebSubtitle)
	}
	return NormalizePanelText(appConfigString("web_subtitle"))
}

// EffectiveAllowXRealIp 是否信任 CDN/反代真实 IP 头。
func EffectiveAllowXRealIp() bool {
	return GetDb().GetGlobalSettings().AllowXRealIp
}

// EffectiveAllowRemoteUpdate 是否允许面板远程升级 npc。
func EffectiveAllowRemoteUpdate() bool {
	return GetDb().GetGlobalSettings().AllowRemoteUpdate
}

// EffectiveAllowWebTerminal 是否开放面板 Web 终端。
func EffectiveAllowWebTerminal() bool {
	return GetDb().GetGlobalSettings().AllowWebTerminal
}

// EffectiveRegisterRateLimit 每 IP 每分钟注册尝试上限；0=不限。
func EffectiveRegisterRateLimit() int {
	return GetDb().GetGlobalSettings().RegisterRateLimit
}

// EffectiveRegisterEmailVerify 注册是否强制邮箱验证码。
func EffectiveRegisterEmailVerify() bool {
	return GetDb().GetGlobalSettings().RegisterEmailVerify
}

// EffectiveHttpErrorPageBrand 错误页品牌名（空=调用方回退 nps）。
func EffectiveHttpErrorPageBrand() string {
	return strings.TrimSpace(GetDb().GetGlobalSettings().HttpErrorPageBrand)
}

// EffectiveHttpErrorPageBrandURL 错误页品牌链接。
func EffectiveHttpErrorPageBrandURL() string {
	return strings.TrimSpace(GetDb().GetGlobalSettings().HttpErrorPageBrandURL)
}
