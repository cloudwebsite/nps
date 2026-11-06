package controllers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/runtimeopts"
)

type GlobalController struct {
	BaseController
}

// geoFileStatus 面板展示的单个 geodata 文件状态。
type geoFileStatus struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Size      int64  `json:"size"`
	SizeHuman string `json:"size_human"`
	ModTime   string `json:"mod_time"`
}

// formatFileSize 按大小选 B / KB / MB / GB。
func formatFileSize(n int64) string {
	if n < 0 {
		n = 0
	}
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(n) / 1024
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if size >= 100 || i == 0 {
		return fmt.Sprintf("%.0f %s", size, units[i])
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}

// geoDataStatus Geo 数据汇总。
type geoDataStatus struct {
	Version    string          `json:"version"`
	Configured bool            `json:"configured"`
	Loaded     bool            `json:"loaded"`
	Files      []geoFileStatus `json:"files"`
}

var geoUpdating atomic.Bool

func geoStatusSnapshot() geoDataStatus {
	ipPath, sitePath := acl.GeoFiles()
	status := geoDataStatus{
		Configured: ipPath != "" || sitePath != "",
		Loaded:     acl.GeoDataConfigured(),
	}
	if ipPath != "" {
		status.Version = install.GeoDataVersion(ipPath)
	}
	for _, geoFile := range install.GeoFiles(ipPath, sitePath) {
		row := geoFileStatus{Name: geoFile.Name, Path: geoFile.Path}
		if info, err := os.Stat(geoFile.Path); err == nil {
			row.Exists = true
			row.Size = info.Size()
			row.SizeHuman = formatFileSize(info.Size())
			row.ModTime = info.ModTime().Local().Format("2006-01-02 15:04:05")
		}
		status.Files = append(status.Files, row)
	}
	return status
}

func (s *GlobalController) Index() {
	s.Data["menu"] = "global"
	s.SetInfo("global")

	global := file.GetDb().GetGlobal()
	settings := file.GetDb().GetGlobalSettings()
	blackIpList := ""
	whiteIpList := ""
	ipListPriority := 0
	serverUrl := ""
	if global != nil {
		blackIpList = strings.Join(global.BlackIpList, "\r\n")
		whiteIpList = strings.Join(global.WhiteIpList, "\r\n")
		ipListPriority = acl.NormalizePriority(global.IpListPriority)
		serverUrl = global.ServerUrl
	}

	s.Data["globalBlackIpList"] = blackIpList
	s.Data["globalWhiteIpList"] = whiteIpList
	s.Data["globalIpListPriority"] = ipListPriority
	s.Data["appName"] = settings.AppName
	s.Data["webWelcome"] = settings.WebWelcome
	s.Data["webSubtitle"] = settings.WebSubtitle
	s.Data["serverUrl"] = serverUrl
	s.Data["allowPorts"] = settings.AllowPorts
	s.Data["ipLimit"] = settings.IpLimit
	s.Data["showHttpProxyPort"] = settings.ShowHttpProxyPort
	s.Data["allowFlowLimit"] = settings.AllowFlowLimit
	s.Data["allowRateLimit"] = settings.AllowRateLimit
	s.Data["allowConnectionNumLimit"] = settings.AllowConnectionNumLimit
	s.Data["allowTunnelNumLimit"] = settings.AllowTunnelNumLimit
	s.Data["allowMultiIp"] = settings.AllowMultiIp
	s.Data["allowLocalProxy"] = settings.AllowLocalProxy
	s.Data["allowUserLogin"] = settings.AllowUserLogin
	s.Data["allowUserRegister"] = settings.AllowUserRegister
	s.Data["allowUserChangeUsername"] = settings.AllowUserChangeUsername
	s.Data["openCaptcha"] = settings.OpenCaptcha
	s.Data["systemInfoDisplay"] = settings.SystemInfoDisplay
	s.Data["httpAddOriginHeader"] = settings.HttpAddOriginHeader

	s.Data["allowRemoteUpdate"] = settings.AllowRemoteUpdate
	s.Data["allowXRealIp"] = settings.AllowXRealIp
	s.Data["wafEnable"] = settings.WafEnable
	s.Data["wafProtectPanel"] = settings.WafProtectPanel
	s.Data["wafUABlacklistEnable"] = settings.WafUABlacklistEnable
	s.Data["wafBlockForeignEnable"] = settings.WafBlockForeignEnable
	s.Data["wafCustomRulesEnable"] = settings.WafCustomRulesEnable
	s.Data["wafBlockForeignUnknown"] = settings.WafBlockForeignUnknown
	s.Data["wafCountryHeaders"] = settings.WafCountryHeaders

	s.Data["registerRateLimit"] = settings.RegisterRateLimit
	s.Data["registerEmailVerify"] = settings.RegisterEmailVerify
	s.Data["smtpHost"] = settings.SmtpHost
	s.Data["smtpPort"] = settings.SmtpPort
	s.Data["smtpSecurity"] = settings.SmtpSecurity
	s.Data["smtpUser"] = settings.SmtpUser
	s.Data["smtpFrom"] = settings.SmtpFrom
	s.Data["smtpPassSet"] = strings.TrimSpace(settings.SmtpPass) != ""

	s.Data["allowWebTerminal"] = settings.AllowWebTerminal
	s.Data["maxClients"] = settings.MaxClients
	s.Data["maxTunnelsPerClient"] = settings.MaxTunnelsPerClient
	s.Data["maxGlobalConn"] = settings.MaxGlobalConn
	s.Data["releaseApiUrl"] = settings.ReleaseAPIURL
	s.Data["releaseDownloadBase"] = settings.ReleaseDownloadBase
	s.Data["httpErrorPageBrand"] = settings.HttpErrorPageBrand
	s.Data["httpErrorPageBrandUrl"] = settings.HttpErrorPageBrandURL

	s.Data["notifyEnable"] = settings.NotifyEnable
	s.Data["notifyTitlePrefix"] = settings.NotifyTitlePrefix
	s.Data["notifyMinInterval"] = settings.NotifyMinInterval
	s.Data["notifyErrorPageMinInterval"] = settings.NotifyErrorPageMinInterval
	s.Data["notifyQueueSize"] = settings.NotifyQueueSize
	s.Data["notifyHttpTimeout"] = settings.NotifyHTTPTimeout
	s.Data["notifyPublicClient"] = settings.NotifyPublicClient
	s.Data["notifyTelegramChatId"] = settings.NotifyTelegramChatID
	s.Data["notifyWecomSet"] = strings.TrimSpace(settings.NotifyWecomWebhook) != ""
	s.Data["notifyTelegramTokenSet"] = strings.TrimSpace(settings.NotifyTelegramBotToken) != ""
	s.Data["notifyWebhookUrlSet"] = strings.TrimSpace(settings.NotifyWebhookURL) != ""

	s.Data["geo"] = geoStatusSnapshot()

	s.display("global/index")
}

// Save 保存全局参数。global.json 会同时更新内存和文件，保存后相关配置立即生效。
func (s *GlobalController) Save() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "global"
		s.SetInfo("save global")
		s.display()
		return
	}

	global := file.GetDb().GetGlobal()
	if global == nil {
		global = new(file.Glob)
	}
	global.BlackIpList = normalizeACLRules(s.getEscapeString("globalBlackIpList"))
	global.WhiteIpList = normalizeACLRules(s.getEscapeString("globalWhiteIpList"))
	global.IpListPriority = acl.NormalizePriority(s.GetIntNoErr("globalIpListPriority"))
	appName := file.NormalizeAppName(s.GetString("appName"))
	if appName == "" {
		appName = file.DefaultAppName
	}
	global.AppName = appName
	global.ServerUrl = s.getEscapeString("serverUrl")
	global.AllowPorts = globalStringPtr(strings.TrimSpace(s.getEscapeString("allowPorts")))
	global.IpLimit = globalBoolPtr(s.GetBoolNoErr("ipLimit"))
	global.ShowHttpProxyPort = globalBoolPtr(s.GetBoolNoErr("showHttpProxyPort"))
	global.AllowFlowLimit = globalBoolPtr(s.GetBoolNoErr("allowFlowLimit"))
	global.AllowRateLimit = globalBoolPtr(s.GetBoolNoErr("allowRateLimit"))
	global.AllowConnectionNumLimit = globalBoolPtr(s.GetBoolNoErr("allowConnectionNumLimit"))
	global.AllowTunnelNumLimit = globalBoolPtr(s.GetBoolNoErr("allowTunnelNumLimit"))
	global.AllowMultiIp = globalBoolPtr(s.GetBoolNoErr("allowMultiIp"))
	global.AllowLocalProxy = globalBoolPtr(s.GetBoolNoErr("allowLocalProxy"))
	global.AllowUserLogin = globalBoolPtr(s.GetBoolNoErr("allowUserLogin"))
	global.AllowUserRegister = globalBoolPtr(s.GetBoolNoErr("allowUserRegister"))
	global.AllowUserChangeUsername = globalBoolPtr(s.GetBoolNoErr("allowUserChangeUsername"))
	global.OpenCaptcha = globalBoolPtr(s.GetBoolNoErr("openCaptcha"))
	global.SystemInfoDisplay = globalBoolPtr(s.GetBoolNoErr("systemInfoDisplay"))
	global.HttpAddOriginHeader = globalBoolPtr(s.GetBoolNoErr("httpAddOriginHeader"))

	global.WebWelcome = globalStringPtr(file.NormalizePanelText(s.GetString("webWelcome")))
	global.WebSubtitle = globalStringPtr(file.NormalizePanelText(s.GetString("webSubtitle")))
	global.AllowRemoteUpdate = globalBoolPtr(s.GetBoolNoErr("allowRemoteUpdate"))
	global.AllowXRealIp = globalBoolPtr(s.GetBoolNoErr("allowXRealIp"))

	global.WafEnable = globalBoolPtr(s.GetBoolNoErr("wafEnable"))
	global.WafProtectPanel = globalBoolPtr(s.GetBoolNoErr("wafProtectPanel"))
	global.WafUABlacklistEnable = globalBoolPtr(s.GetBoolNoErr("wafUABlacklistEnable"))
	global.WafBlockForeignEnable = globalBoolPtr(s.GetBoolNoErr("wafBlockForeignEnable"))
	global.WafCustomRulesEnable = globalBoolPtr(s.GetBoolNoErr("wafCustomRulesEnable"))
	global.WafBlockForeignUnknown = globalBoolPtr(s.GetBoolNoErr("wafBlockForeignUnknown"))
	headers := strings.TrimSpace(s.GetString("wafCountryHeaders"))
	if headers == "" {
		headers = "CF-IPCountry"
	}
	global.WafCountryHeaders = globalStringPtr(headers)

	global.RegisterRateLimit = globalIntPtr(s.GetIntNoErr("registerRateLimit"))
	global.RegisterEmailVerify = globalBoolPtr(s.GetBoolNoErr("registerEmailVerify"))
	global.SmtpHost = globalStringPtr(strings.TrimSpace(s.GetString("smtpHost")))
	global.SmtpPort = globalIntPtr(s.GetIntNoErr("smtpPort", 587))
	sec := strings.ToLower(strings.TrimSpace(s.GetString("smtpSecurity")))
	if sec == "" {
		sec = "starttls"
	}
	global.SmtpSecurity = globalStringPtr(sec)
	global.SmtpUser = globalStringPtr(strings.TrimSpace(s.GetString("smtpUser")))
	global.SmtpFrom = globalStringPtr(strings.TrimSpace(s.GetString("smtpFrom")))
	if pass := s.GetString("smtpPass"); strings.TrimSpace(pass) != "" {
		global.SmtpPass = globalStringPtr(pass)
	} else if global.SmtpPass == nil {
		global.SmtpPass = globalStringPtr("")
	}

	global.AllowWebTerminal = globalBoolPtr(s.GetBoolNoErr("allowWebTerminal"))
	global.MaxClients = globalIntPtr(s.GetIntNoErr("maxClients"))
	global.MaxTunnelsPerClient = globalIntPtr(s.GetIntNoErr("maxTunnelsPerClient"))
	global.MaxGlobalConn = globalIntPtr(s.GetIntNoErr("maxGlobalConn"))
	global.ReleaseAPIURL = globalStringPtr(strings.TrimSpace(s.GetString("releaseApiUrl")))
	global.ReleaseDownloadBase = globalStringPtr(strings.TrimSpace(s.GetString("releaseDownloadBase")))
	global.HttpErrorPageBrand = globalStringPtr(file.NormalizePanelText(s.GetString("httpErrorPageBrand")))
	global.HttpErrorPageBrandURL = globalStringPtr(strings.TrimSpace(s.GetString("httpErrorPageBrandUrl")))

	global.NotifyEnable = globalBoolPtr(s.GetBoolNoErr("notifyEnable"))
	prefix := strings.TrimSpace(s.GetString("notifyTitlePrefix"))
	if prefix == "" {
		prefix = "[nps]"
	}
	global.NotifyTitlePrefix = globalStringPtr(prefix)
	global.NotifyMinInterval = globalIntPtr(s.GetIntNoErr("notifyMinInterval", 30))
	global.NotifyErrorPageMinInterval = globalIntPtr(s.GetIntNoErr("notifyErrorPageMinInterval", 300))
	global.NotifyQueueSize = globalIntPtr(s.GetIntNoErr("notifyQueueSize", 64))
	global.NotifyHTTPTimeout = globalIntPtr(s.GetIntNoErr("notifyHttpTimeout", 5))
	global.NotifyPublicClient = globalBoolPtr(s.GetBoolNoErr("notifyPublicClient"))
	global.NotifyTelegramChatID = globalStringPtr(strings.TrimSpace(s.GetString("notifyTelegramChatId")))
	if v := s.GetString("notifyWecomWebhook"); strings.TrimSpace(v) != "" {
		global.NotifyWecomWebhook = globalStringPtr(strings.TrimSpace(v))
	} else if global.NotifyWecomWebhook == nil {
		global.NotifyWecomWebhook = globalStringPtr("")
	}
	if v := s.GetString("notifyTelegramBotToken"); strings.TrimSpace(v) != "" {
		global.NotifyTelegramBotToken = globalStringPtr(strings.TrimSpace(v))
	} else if global.NotifyTelegramBotToken == nil {
		global.NotifyTelegramBotToken = globalStringPtr("")
	}
	if v := s.GetString("notifyWebhookUrl"); strings.TrimSpace(v) != "" {
		global.NotifyWebhookURL = globalStringPtr(strings.TrimSpace(v))
	} else if global.NotifyWebhookURL == nil {
		global.NotifyWebhookURL = globalStringPtr("")
	}
	if v := s.GetString("notifyWebhookHeader"); strings.TrimSpace(v) != "" {
		global.NotifyWebhookHeader = globalStringPtr(v)
	} else if global.NotifyWebhookHeader == nil {
		global.NotifyWebhookHeader = globalStringPtr("")
	}

	if err := file.GetDb().SaveGlobal(global); err != nil {
		s.AjaxErr(err.Error())
		return
	}
	runtimeopts.Apply()
	s.AjaxOk("save success")
}

func globalIntPtr(v int) *int {
	return &v
}

// GeoStatus 返回当前 Geo 数据状态（不联网）。
func (s *GlobalController) GeoStatus() {
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
		return
	}
	s.Data["json"] = map[string]interface{}{
		"status": 1,
		"msg":    "ok",
		"data":   geoStatusSnapshot(),
	}
	s.ServeJSON()
	s.StopRun()
}

// UpdateGeoData 在线拉取 Loyalsoldier geoip/geosite 并热加载。
func (s *GlobalController) UpdateGeoData() {
	if s.Ctx.Request.Method != "POST" {
		s.AjaxErr("method not allowed")
		return
	}
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
		return
	}
	if !geoUpdating.CompareAndSwap(false, true) {
		s.AjaxErr("Geo 数据更新正在进行中，请稍候")
		return
	}
	defer geoUpdating.Store(false)

	ipPath, sitePath := acl.GeoFiles()
	if ipPath == "" && sitePath == "" {
		s.AjaxErr("未配置 Geo 数据路径，无法在线更新")
		return
	}
	result, err := install.UpdateGeoData(ipPath, sitePath)
	if err != nil {
		s.auditPanel("global.geodata.update", "geodata", 0, 0, "failure", err.Error(), nil)
		s.AjaxErr("Geo 数据更新失败: " + err.Error())
		return
	}
	if result != nil && len(result.Updated) > 0 {
		acl.ReloadGeoData()
	}
	version := ""
	updated := 0
	if result != nil {
		version = result.Version
		updated = len(result.Updated)
	}
	s.auditPanel("global.geodata.update", "geodata", 0, 0, "success", "", map[string]string{
		"version": version,
		"updated": strconv.Itoa(updated),
	})
	message := "Geo 数据已是最新"
	if updated > 0 {
		message = fmt.Sprintf("Geo 数据更新成功，已更新 %d 个文件", updated)
	}
	s.Data["json"] = map[string]interface{}{
		"status": 1,
		"msg":    message,
		"data":   geoStatusSnapshot(),
	}
	s.ServeJSON()
	s.StopRun()
}

func globalBoolPtr(value bool) *bool {
	return &value
}

func globalStringPtr(value string) *string {
	return &value
}
