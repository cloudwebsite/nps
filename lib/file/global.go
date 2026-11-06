package file

import (
	"strings"
	"unicode/utf8"

	"github.com/astaxie/beego"
)

const DefaultAppName = "NPS"
const maxAppNameRunes = 32

// GlobalSettings 是 Glob 的只读快照：把尚未迁移的指针字段解析成带默认值的普通值，
// 方便在请求/连接处理中直接读取，保证 Web 保存后立即生效。
type GlobalSettings struct {
	AppName                 string
	AllowPorts              string
	IpLimit                 bool
	ShowHttpProxyPort       bool
	AllowFlowLimit          bool
	AllowRateLimit          bool
	AllowConnectionNumLimit bool
	AllowTunnelNumLimit     bool
	AllowMultiIp            bool
	AllowLocalProxy         bool
	AllowUserLogin          bool
	AllowUserRegister       bool
	AllowUserChangeUsername bool
	OpenCaptcha             bool
	SystemInfoDisplay       bool
	HttpAddOriginHeader     bool

	WebWelcome  string
	WebSubtitle string

	AllowRemoteUpdate bool
	AllowXRealIp      bool

	WafEnable              bool
	WafProtectPanel        bool
	WafUABlacklistEnable   bool
	WafBlockForeignEnable  bool
	WafCustomRulesEnable   bool
	WafBlockForeignUnknown bool
	WafCountryHeaders      string

	RegisterRateLimit   int
	RegisterEmailVerify bool
	SmtpHost            string
	SmtpPort            int
	SmtpSecurity        string
	SmtpUser            string
	SmtpPass            string
	SmtpFrom            string

	AllowWebTerminal bool

	NotifyEnable               bool
	NotifyTitlePrefix          string
	NotifyMinInterval          int
	NotifyErrorPageMinInterval int
	NotifyQueueSize            int
	NotifyHTTPTimeout          int
	NotifyPublicClient         bool
	NotifyWecomWebhook         string
	NotifyTelegramBotToken     string
	NotifyTelegramChatID       string
	NotifyWebhookURL           string
	NotifyWebhookHeader        string

	HttpErrorPageBrand    string
	HttpErrorPageBrandURL string

	MaxClients          int
	MaxTunnelsPerClient int
	MaxGlobalConn       int

	ReleaseAPIURL       string
	ReleaseDownloadBase string
}

// NormalizeAppName 裁剪空白、危险字符与长度；空串表示未设置（调用方再回退默认名）。
func NormalizeAppName(raw string) string {
	name := strings.TrimSpace(raw)
	if name == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r < 32 || r == '"' || r == '\'' || r == '<' || r == '>' || r == '\\' || r == '`' || r == '&' {
			continue
		}
		b.WriteRune(r)
	}
	name = strings.TrimSpace(b.String())
	if name == "" {
		return ""
	}
	if utf8.RuneCountInString(name) > maxAppNameRunes {
		runes := []rune(name)
		name = string(runes[:maxAppNameRunes])
	}
	return name
}

// EffectiveAppName 面板展示用产品名：global.json → nps.conf appname → NPS。
func EffectiveAppName() string {
	if global := GetDb().GetGlobal(); global != nil {
		if s := NormalizeAppName(global.AppName); s != "" {
			return s
		}
	}
	if s := NormalizeAppName(beego.AppConfig.String("appname")); s != "" {
		return s
	}
	return DefaultAppName
}

func boolPtrValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}

func stringPtrValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// GetGlobalSettings 返回全局参数的只读快照，未初始化的字段使用历史默认值 / conf 回退。
func (s *DbUtils) GetGlobalSettings() GlobalSettings {
	settings := GlobalSettings{
		AppName:                 DefaultAppName,
		ShowHttpProxyPort:       true,
		AllowFlowLimit:          true,
		AllowRateLimit:          true,
		AllowConnectionNumLimit: true,
		AllowTunnelNumLimit:     true,
		AllowMultiIp:            true,
		AllowUserLogin:          true,
		AllowUserChangeUsername: true,
		SystemInfoDisplay:       true,
		HttpAddOriginHeader:     true,
		AllowXRealIp:            true,
		WafProtectPanel:         true,
		WafBlockForeignUnknown:  true,
		WafCountryHeaders:       "CF-IPCountry",
		RegisterRateLimit:       5,
		SmtpPort:                587,
		SmtpSecurity:            "starttls",
		NotifyTitlePrefix:       "[nps]",
		NotifyMinInterval:       30,
		NotifyErrorPageMinInterval: 300,
		NotifyQueueSize:         64,
		NotifyHTTPTimeout:       5,
	}
	global := s.GetGlobal()
	if global == nil {
		// 无 global 时仍可读 conf，便于迁移前启动路径
		fillSettingsFromConf(&settings)
		return settings
	}
	if name := NormalizeAppName(global.AppName); name != "" {
		settings.AppName = name
	} else if confName := NormalizeAppName(beego.AppConfig.String("appname")); confName != "" {
		settings.AppName = confName
	}
	settings.AllowPorts = stringPtrValue(global.AllowPorts)
	settings.IpLimit = boolPtrValue(global.IpLimit)
	settings.ShowHttpProxyPort = boolPtrOr(global.ShowHttpProxyPort, true)
	settings.AllowFlowLimit = boolPtrOr(global.AllowFlowLimit, true)
	settings.AllowRateLimit = boolPtrOr(global.AllowRateLimit, true)
	settings.AllowConnectionNumLimit = boolPtrOr(global.AllowConnectionNumLimit, true)
	settings.AllowTunnelNumLimit = boolPtrOr(global.AllowTunnelNumLimit, true)
	settings.AllowMultiIp = boolPtrOr(global.AllowMultiIp, true)
	settings.AllowLocalProxy = boolPtrValue(global.AllowLocalProxy)
	settings.AllowUserLogin = boolPtrOr(global.AllowUserLogin, true)
	settings.AllowUserRegister = boolPtrValue(global.AllowUserRegister)
	settings.AllowUserChangeUsername = boolPtrOr(global.AllowUserChangeUsername, true)
	settings.OpenCaptcha = boolPtrValue(global.OpenCaptcha)
	settings.SystemInfoDisplay = boolPtrOr(global.SystemInfoDisplay, true)
	settings.HttpAddOriginHeader = boolPtrOr(global.HttpAddOriginHeader, true)

	settings.WebWelcome = NormalizePanelText(stringPtrOr(global.WebWelcome, appConfigString("web_welcome")))
	settings.WebSubtitle = NormalizePanelText(stringPtrOr(global.WebSubtitle, appConfigString("web_subtitle")))

	settings.AllowRemoteUpdate = boolPtrOr(global.AllowRemoteUpdate, appConfigBool("allow_remote_update", false))
	settings.AllowXRealIp = boolPtrOr(global.AllowXRealIp, appConfigBool("allow_x_real_ip", true))

	settings.WafEnable = boolPtrOr(global.WafEnable, appConfigBool("waf_enable", false))
	settings.WafProtectPanel = boolPtrOr(global.WafProtectPanel, appConfigBool("waf_protect_panel", true))
	settings.WafUABlacklistEnable = boolPtrOr(global.WafUABlacklistEnable, appConfigBool("waf_ua_blacklist_enable", false))
	settings.WafBlockForeignEnable = boolPtrOr(global.WafBlockForeignEnable, appConfigBool("waf_block_foreign_enable", false))
	settings.WafCustomRulesEnable = boolPtrOr(global.WafCustomRulesEnable, appConfigBool("waf_custom_rules_enable", false))
	settings.WafBlockForeignUnknown = boolPtrOr(global.WafBlockForeignUnknown, appConfigBool("waf_block_foreign_unknown", true))
	settings.WafCountryHeaders = strings.TrimSpace(stringPtrOr(global.WafCountryHeaders, firstNonEmpty(appConfigString("waf_country_headers"), "CF-IPCountry")))

	settings.RegisterRateLimit = intPtrValue(global.RegisterRateLimit, appConfigInt("register_rate_limit", 5))
	settings.RegisterEmailVerify = boolPtrOr(global.RegisterEmailVerify, appConfigBool("register_email_verify", false))
	settings.SmtpHost = strings.TrimSpace(stringPtrOr(global.SmtpHost, firstNonEmpty(appConfigString("smtp_host"), "")))
	settings.SmtpPort = intPtrValue(global.SmtpPort, appConfigInt("smtp_port", 587))
	settings.SmtpSecurity = strings.ToLower(strings.TrimSpace(stringPtrOr(global.SmtpSecurity, firstNonEmpty(appConfigString("smtp_security"), "starttls"))))
	settings.SmtpUser = strings.TrimSpace(stringPtrOr(global.SmtpUser, firstNonEmpty(appConfigString("smtp_user"), appConfigString("smtp_username"))))
	settings.SmtpPass = stringPtrOr(global.SmtpPass, firstNonEmpty(appConfigString("smtp_pass"), appConfigString("smtp_password")))
	settings.SmtpFrom = strings.TrimSpace(stringPtrOr(global.SmtpFrom, appConfigString("smtp_from")))

	settings.AllowWebTerminal = boolPtrOr(global.AllowWebTerminal, appConfigBool("allow_web_terminal", false))

	settings.NotifyEnable = boolPtrOr(global.NotifyEnable, appConfigBool("notify_enable", false))
	settings.NotifyTitlePrefix = stringPtrOr(global.NotifyTitlePrefix, firstNonEmpty(strings.TrimSpace(appConfigString("notify_title_prefix")), "[nps]"))
	settings.NotifyMinInterval = intPtrValue(global.NotifyMinInterval, appConfigInt("notify_min_interval", 30))
	settings.NotifyErrorPageMinInterval = intPtrValue(global.NotifyErrorPageMinInterval, appConfigInt("notify_error_page_min_interval", 300))
	settings.NotifyQueueSize = intPtrValue(global.NotifyQueueSize, appConfigInt("notify_queue_size", 64))
	settings.NotifyHTTPTimeout = intPtrValue(global.NotifyHTTPTimeout, appConfigInt("notify_http_timeout", 5))
	settings.NotifyPublicClient = boolPtrOr(global.NotifyPublicClient, appConfigBool("notify_public_client", false))
	settings.NotifyWecomWebhook = stringPtrOr(global.NotifyWecomWebhook, appConfigString("notify_wecom_webhook"))
	settings.NotifyTelegramBotToken = stringPtrOr(global.NotifyTelegramBotToken, appConfigString("notify_telegram_bot_token"))
	settings.NotifyTelegramChatID = stringPtrOr(global.NotifyTelegramChatID, appConfigString("notify_telegram_chat_id"))
	settings.NotifyWebhookURL = stringPtrOr(global.NotifyWebhookURL, appConfigString("notify_webhook_url"))
	settings.NotifyWebhookHeader = stringPtrOr(global.NotifyWebhookHeader, appConfigString("notify_webhook_header"))

	settings.HttpErrorPageBrand = strings.TrimSpace(stringPtrOr(global.HttpErrorPageBrand, appConfigString("http_error_page_brand")))
	settings.HttpErrorPageBrandURL = strings.TrimSpace(stringPtrOr(global.HttpErrorPageBrandURL, appConfigString("http_error_page_brand_url")))

	settings.MaxClients = intPtrValue(global.MaxClients, appConfigInt("max_clients", 0))
	settings.MaxTunnelsPerClient = intPtrValue(global.MaxTunnelsPerClient, appConfigInt("max_tunnels_per_client", 0))
	settings.MaxGlobalConn = intPtrValue(global.MaxGlobalConn, appConfigInt("max_global_conn", 0))

	settings.ReleaseAPIURL = strings.TrimSpace(stringPtrOr(global.ReleaseAPIURL, appConfigString("release_api_url")))
	settings.ReleaseDownloadBase = strings.TrimSpace(stringPtrOr(global.ReleaseDownloadBase, appConfigString("release_download_base")))
	return settings
}

func fillSettingsFromConf(settings *GlobalSettings) {
	settings.WebWelcome = NormalizePanelText(appConfigString("web_welcome"))
	settings.WebSubtitle = NormalizePanelText(appConfigString("web_subtitle"))
	settings.AllowRemoteUpdate = appConfigBool("allow_remote_update", false)
	settings.AllowXRealIp = appConfigBool("allow_x_real_ip", true)
	settings.WafEnable = appConfigBool("waf_enable", false)
	settings.WafProtectPanel = appConfigBool("waf_protect_panel", true)
	settings.WafUABlacklistEnable = appConfigBool("waf_ua_blacklist_enable", false)
	settings.WafBlockForeignEnable = appConfigBool("waf_block_foreign_enable", false)
	settings.WafCustomRulesEnable = appConfigBool("waf_custom_rules_enable", false)
	settings.WafBlockForeignUnknown = appConfigBool("waf_block_foreign_unknown", true)
	settings.WafCountryHeaders = firstNonEmpty(strings.TrimSpace(appConfigString("waf_country_headers")), "CF-IPCountry")
	settings.RegisterRateLimit = appConfigInt("register_rate_limit", 5)
	settings.RegisterEmailVerify = appConfigBool("register_email_verify", false)
	settings.AllowWebTerminal = appConfigBool("allow_web_terminal", false)
	settings.MaxClients = appConfigInt("max_clients", 0)
	settings.MaxTunnelsPerClient = appConfigInt("max_tunnels_per_client", 0)
	settings.MaxGlobalConn = appConfigInt("max_global_conn", 0)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func appConfigBool(key string, defaultValue bool) bool {
	value, err := beego.AppConfig.Bool(key)
	if err != nil {
		return defaultValue
	}
	return value
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func migrateStringPtr(dst **string, confKey string, changed *bool) {
	if *dst != nil {
		return
	}
	*dst = stringPtr(beego.AppConfig.String(confKey))
	*changed = true
}

func migrateBoolPtr(dst **bool, confKey string, def bool, changed *bool) {
	if *dst != nil {
		return
	}
	*dst = boolPtr(appConfigBool(confKey, def))
	*changed = true
}

func migrateIntPtr(dst **int, confKey string, def int, changed *bool) {
	if *dst != nil {
		return
	}
	*dst = intPtr(appConfigInt(confKey, def))
	*changed = true
}

// MigrateGlobalFromAppConfig 在启动时把 nps.conf 中可热更新的参数补齐到 global.json。
// 只填充仍未初始化的字段：已经通过 Web 页面保存过的值不会被 nps.conf 覆盖。
func (s *DbUtils) MigrateGlobalFromAppConfig() {
	global := s.GetGlobal()
	if global == nil {
		global = new(Glob)
	}
	if global.BlackIpList == nil {
		global.BlackIpList = []string{}
	}
	changed := false

	if NormalizeAppName(global.AppName) == "" {
		if confName := NormalizeAppName(beego.AppConfig.String("appname")); confName != "" {
			global.AppName = confName
			changed = true
		} else {
			global.AppName = DefaultAppName
			changed = true
		}
	}

	if global.AllowPorts == nil {
		global.AllowPorts = stringPtr(beego.AppConfig.String("allow_ports"))
		changed = true
	}
	migrateBoolPtr(&global.IpLimit, "ip_limit", false, &changed)
	migrateBoolPtr(&global.ShowHttpProxyPort, "show_http_proxy_port", true, &changed)
	migrateBoolPtr(&global.AllowFlowLimit, "allow_flow_limit", true, &changed)
	migrateBoolPtr(&global.AllowRateLimit, "allow_rate_limit", true, &changed)
	migrateBoolPtr(&global.AllowConnectionNumLimit, "allow_connection_num_limit", true, &changed)
	migrateBoolPtr(&global.AllowTunnelNumLimit, "allow_tunnel_num_limit", true, &changed)
	migrateBoolPtr(&global.AllowMultiIp, "allow_multi_ip", true, &changed)
	migrateBoolPtr(&global.AllowLocalProxy, "allow_local_proxy", false, &changed)
	migrateBoolPtr(&global.AllowUserLogin, "allow_user_login", true, &changed)
	migrateBoolPtr(&global.AllowUserRegister, "allow_user_register", false, &changed)
	migrateBoolPtr(&global.AllowUserChangeUsername, "allow_user_change_username", true, &changed)
	migrateBoolPtr(&global.OpenCaptcha, "open_captcha", false, &changed)
	migrateBoolPtr(&global.SystemInfoDisplay, "system_info_display", true, &changed)
	migrateBoolPtr(&global.HttpAddOriginHeader, "http_add_origin_header", true, &changed)

	migrateStringPtr(&global.WebWelcome, "web_welcome", &changed)
	migrateStringPtr(&global.WebSubtitle, "web_subtitle", &changed)
	migrateBoolPtr(&global.AllowRemoteUpdate, "allow_remote_update", false, &changed)
	migrateBoolPtr(&global.AllowXRealIp, "allow_x_real_ip", true, &changed)

	migrateBoolPtr(&global.WafEnable, "waf_enable", false, &changed)
	migrateBoolPtr(&global.WafProtectPanel, "waf_protect_panel", true, &changed)
	migrateBoolPtr(&global.WafUABlacklistEnable, "waf_ua_blacklist_enable", false, &changed)
	migrateBoolPtr(&global.WafBlockForeignEnable, "waf_block_foreign_enable", false, &changed)
	migrateBoolPtr(&global.WafCustomRulesEnable, "waf_custom_rules_enable", false, &changed)
	migrateBoolPtr(&global.WafBlockForeignUnknown, "waf_block_foreign_unknown", true, &changed)
	if global.WafCountryHeaders == nil {
		h := strings.TrimSpace(beego.AppConfig.String("waf_country_headers"))
		if h == "" {
			h = "CF-IPCountry"
		}
		global.WafCountryHeaders = stringPtr(h)
		changed = true
	}

	migrateIntPtr(&global.RegisterRateLimit, "register_rate_limit", 5, &changed)
	migrateBoolPtr(&global.RegisterEmailVerify, "register_email_verify", false, &changed)
	migrateStringPtr(&global.SmtpHost, "smtp_host", &changed)
	migrateIntPtr(&global.SmtpPort, "smtp_port", 587, &changed)
	if global.SmtpSecurity == nil {
		sec := strings.TrimSpace(beego.AppConfig.String("smtp_security"))
		if sec == "" {
			sec = "starttls"
		}
		global.SmtpSecurity = stringPtr(sec)
		changed = true
	}
	if global.SmtpUser == nil {
		u := beego.AppConfig.String("smtp_user")
		if strings.TrimSpace(u) == "" {
			u = beego.AppConfig.String("smtp_username")
		}
		global.SmtpUser = stringPtr(u)
		changed = true
	}
	if global.SmtpPass == nil {
		p := beego.AppConfig.String("smtp_pass")
		if strings.TrimSpace(p) == "" {
			p = beego.AppConfig.String("smtp_password")
		}
		global.SmtpPass = stringPtr(p)
		changed = true
	}
	migrateStringPtr(&global.SmtpFrom, "smtp_from", &changed)

	migrateBoolPtr(&global.AllowWebTerminal, "allow_web_terminal", false, &changed)

	migrateBoolPtr(&global.NotifyEnable, "notify_enable", false, &changed)
	if global.NotifyTitlePrefix == nil {
		p := strings.TrimSpace(beego.AppConfig.String("notify_title_prefix"))
		if p == "" {
			p = "[nps]"
		}
		global.NotifyTitlePrefix = stringPtr(p)
		changed = true
	}
	migrateIntPtr(&global.NotifyMinInterval, "notify_min_interval", 30, &changed)
	migrateIntPtr(&global.NotifyErrorPageMinInterval, "notify_error_page_min_interval", 300, &changed)
	migrateIntPtr(&global.NotifyQueueSize, "notify_queue_size", 64, &changed)
	migrateIntPtr(&global.NotifyHTTPTimeout, "notify_http_timeout", 5, &changed)
	migrateBoolPtr(&global.NotifyPublicClient, "notify_public_client", false, &changed)
	migrateStringPtr(&global.NotifyWecomWebhook, "notify_wecom_webhook", &changed)
	migrateStringPtr(&global.NotifyTelegramBotToken, "notify_telegram_bot_token", &changed)
	migrateStringPtr(&global.NotifyTelegramChatID, "notify_telegram_chat_id", &changed)
	migrateStringPtr(&global.NotifyWebhookURL, "notify_webhook_url", &changed)
	migrateStringPtr(&global.NotifyWebhookHeader, "notify_webhook_header", &changed)

	migrateStringPtr(&global.HttpErrorPageBrand, "http_error_page_brand", &changed)
	migrateStringPtr(&global.HttpErrorPageBrandURL, "http_error_page_brand_url", &changed)

	migrateIntPtr(&global.MaxClients, "max_clients", 0, &changed)
	migrateIntPtr(&global.MaxTunnelsPerClient, "max_tunnels_per_client", 0, &changed)
	migrateIntPtr(&global.MaxGlobalConn, "max_global_conn", 0, &changed)

	migrateStringPtr(&global.ReleaseAPIURL, "release_api_url", &changed)
	migrateStringPtr(&global.ReleaseDownloadBase, "release_download_base", &changed)

	if changed {
		_ = s.SaveGlobal(global)
	}
}
