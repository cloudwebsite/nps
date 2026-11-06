package controllers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
)

type certStatusView struct {
	Scope             string
	Status            string
	Detail            string
	Valid             bool
	Configured        bool
	Complete          bool
	CertFilePath      string
	KeyFilePath       string
	Subject           string
	Issuer            string
	ExpiresAt         string
	DaysRemaining     int
	Fingerprint       string
	RuntimeConfigured bool
	RuntimeStatus     string
}

func buildWebCertStatus(values map[string]string) certStatusView {
	status := certStatusView{
		Scope:        "web",
		CertFilePath: strings.TrimSpace(values["web_cert_file"]),
		KeyFilePath:  strings.TrimSpace(values["web_key_file"]),
	}
	status.Configured = status.CertFilePath != "" || status.KeyFilePath != ""
	status.Complete = status.CertFilePath != "" && status.KeyFilePath != ""
	inspection, configuredStatus, err := inspectConfiguredCertificateFiles(status.CertFilePath, status.KeyFilePath, "")
	status.Status = configuredStatus
	status.Valid = inspection.Valid
	status.Subject = inspection.Subject
	status.Issuer = inspection.Issuer
	status.ExpiresAt = inspection.ExpiresAt
	status.DaysRemaining = inspection.DaysRemaining
	status.Fingerprint = inspection.Fingerprint
	if err != nil {
		status.Detail = firstCertLine(err.Error())
	}
	if !status.Configured {
		status.Status = "未配置面板证书"
	}
	return status
}

type acmeStatusView struct {
	CA               string
	CachePath        string
	Force            bool
	EmailSet         bool
	ZeroSSLKeySet    bool
	PortReady        bool
	AutoSSLHostCount int
	Summary          string
	Detail           string
}

// buildAcmeStatus 证书页 ACME 配置摘要；不依赖运行中 manager。
func buildAcmeStatus(values map[string]string) acmeStatusView {
	if values == nil {
		values = map[string]string{}
	}
	caRaw := strings.ToLower(strings.TrimSpace(values["ssl_ca"]))
	ca := "Let's Encrypt"
	if caRaw == "zerossl" || caRaw == "zero_ssl" || caRaw == "zero ssl" {
		ca = "ZeroSSL"
	} else if caRaw != "" && caRaw != "letsencrypt" && caRaw != "let's encrypt" {
		ca = caRaw
	}
	cachePath := strings.TrimSpace(values["ssl_path"])
	if cachePath == "" {
		cachePath = filepath.Join(common.GetRunPath(), "conf", "ssl")
	} else if !filepath.IsAbs(cachePath) {
		cachePath = filepath.Join(common.GetRunPath(), cachePath)
	}
	force := strings.EqualFold(strings.TrimSpace(values["force_auto_ssl"]), "true")
	httpPort := beego.AppConfig.DefaultInt("http_proxy_port", 80)
	httpsPort := beego.AppConfig.DefaultInt("https_proxy_port", 443)
	if v := strings.TrimSpace(values["http_proxy_port"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			httpPort = n
		}
	}
	if v := strings.TrimSpace(values["https_proxy_port"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			httpsPort = n
		}
	}
	portReady := force || httpPort == 80 || httpsPort == 443
	autoCount := 0
	if db := file.GetDb(); db != nil && db.JsonDb != nil {
		db.JsonDb.Hosts.Range(func(_, value interface{}) bool {
			if h, ok := value.(*file.Host); ok && h != nil && h.AutoSSL && !h.IsClose {
				autoCount++
			}
			return true
		})
	}
	status := acmeStatusView{
		CA:               ca,
		CachePath:        cachePath,
		Force:            force,
		EmailSet:         strings.TrimSpace(values["ssl_email"]) != "",
		ZeroSSLKeySet:    strings.TrimSpace(values["ssl_zerossl_api"]) != "",
		PortReady:        portReady,
		AutoSSLHostCount: autoCount,
	}
	switch {
	case !portReady:
		status.Summary = "未就绪：非 80/443 且未开 force_auto_ssl"
	case ca == "ZeroSSL" && !status.ZeroSSLKeySet:
		status.Summary = "未就绪：ZeroSSL 需配置 ssl_zerossl_api"
	case autoCount == 0:
		status.Summary = "配置可启用；尚无 Host 开启 AutoSSL"
	default:
		status.Summary = "可申请：已有 AutoSSL Host"
	}
	status.Detail = "Host 须显式 AutoSSL；开启后优先于手工证书路径；改 CA/密钥后需重启。"
	return status
}

func buildBridgeCertStatus(values map[string]string) certStatusView {
	status := certStatusView{
		Scope:             "bridge",
		CertFilePath:      strings.TrimSpace(values["bridge_cert_file"]),
		KeyFilePath:       strings.TrimSpace(values["bridge_key_file"]),
		RuntimeConfigured: crypt.IsCertLoadedFromFiles(),
	}
	status.Configured = status.CertFilePath != "" || status.KeyFilePath != ""
	status.Complete = status.CertFilePath != "" && status.KeyFilePath != ""
	inspection, configuredStatus, err := inspectConfiguredCertificateFiles(status.CertFilePath, status.KeyFilePath, "")
	status.Status = configuredStatus
	status.Valid = inspection.Valid
	status.Subject = inspection.Subject
	status.Issuer = inspection.Issuer
	status.ExpiresAt = inspection.ExpiresAt
	status.DaysRemaining = inspection.DaysRemaining
	status.Fingerprint = inspection.Fingerprint
	if err != nil {
		status.Detail = firstCertLine(err.Error())
	}
	if status.RuntimeConfigured {
		status.RuntimeStatus = "运行中：文件证书"
	} else {
		status.RuntimeStatus = "运行中：临时自签名"
	}
	// 展示进程内实际证书指纹（含临时自签名），便于运维核对 / 后续客户端固定
	if fp := crypt.GetCertFingerprint(); fp != "" {
		status.Fingerprint = fp
		if status.Detail == "" {
			status.Detail = "运行态 SHA-256 指纹可用于客户端 tls_fingerprint（默认不强制）"
		}
	}
	if !status.Configured {
		status.Status = "未配置 Bridge 证书（重启后使用临时自签名）"
	}
	return status
}

// CheckCertificate POST：仅校验，不写盘。
func (s *ServerConfigController) CheckCertificate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(s.GetString("mode")))
	if mode == "" {
		mode = "path"
	}
	certPEM, keyPEM, err := certificateMaterialFromRequest(s.Ctx.Request, mode)
	if err != nil {
		s.AjaxErr(err.Error())
		return
	}
	hostname := strings.TrimSpace(s.GetString("hostname"))
	wildcard := s.GetBoolNoErr("wildcard")
	inspection, inspectErr := inspectCertificatePair(certPEM, keyPEM, hostname, wildcard)
	if inspectErr != nil {
		inspection.Message = inspectErr.Error()
	}
	s.Data["json"] = map[string]interface{}{
		"status": 1,
		"msg":    inspection.Message,
		"data":   inspection,
	}
	s.ServeJSON()
	s.StopRun()
}

func (s *ServerConfigController) saveScopedCertificate(persistScope, label, certKey, keyKey, auditAction string) {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	mode := strings.ToLower(strings.TrimSpace(s.GetString("mode")))
	if mode == "" {
		mode = "path"
	}
	hostname := strings.TrimSpace(s.GetString("hostname"))
	path := serverConfigPath()
	if _, err := os.Stat(path); err != nil {
		s.AjaxErr("读取 nps.conf 失败: " + err.Error())
		return
	}

	var updates []confUpdate
	materialChanged := false
	certPath, keyPath := "", ""
	if mode == "clear" {
		updates = []confUpdate{
			{Key: certKey, Omit: true},
			{Key: keyKey, Omit: true},
		}
	} else {
		certPEM, keyPEM, materialErr := certificateMaterialFromRequest(s.Ctx.Request, mode)
		if materialErr != nil {
			s.AjaxErr(label + " 证书无效: " + materialErr.Error())
			return
		}
		if _, inspectErr := inspectCertificatePair(certPEM, keyPEM, hostname, false); inspectErr != nil {
			s.AjaxErr(label + " 证书无效: " + inspectErr.Error())
			return
		}
		if mode == "path" {
			certPath = strings.TrimSpace(s.Ctx.Request.FormValue("cert"))
			keyPath = strings.TrimSpace(s.Ctx.Request.FormValue("key"))
		} else {
			var persistErr error
			certPath, keyPath, persistErr = persistCertificatePair(persistScope, "server", certPEM, keyPEM)
			if persistErr != nil {
				s.AjaxErr(label + " 证书保存失败: " + persistErr.Error())
				return
			}
			materialChanged = true
		}
		if strings.ContainsAny(certPath+keyPath, "\r\n") {
			s.AjaxErr(label + " 证书路径不能包含换行")
			return
		}
		updates = []confUpdate{
			{Key: certKey, Value: certPath},
			{Key: keyKey, Value: keyPath},
		}
	}

	changed, err := writeServerConfigUpdates(path, updates)
	if err != nil {
		s.auditPanel(auditAction, "nps.conf", 0, 0, "failure", err.Error(), nil)
		s.AjaxErr("保存 " + label + " 证书配置失败: " + err.Error())
		return
	}
	restartRequired := len(changed) > 0 || materialChanged
	msg := label + " 证书配置已保存，需要重启 NPS 后生效"
	if !restartRequired {
		msg = label + " 证书配置未发生变化"
	}
	details := map[string]string{
		"mode":             mode,
		"changed":          strconv.Itoa(len(changed)),
		"restart_required": strconv.FormatBool(restartRequired),
	}
	s.auditPanel(auditAction, "nps.conf", 0, 0, "success", "", details)
	s.Data["json"] = map[string]interface{}{
		"status":           1,
		"msg":              msg,
		"changed":          changed,
		"restart_required": restartRequired,
	}
	s.ServeJSON()
	s.StopRun()
}

// SaveWebCertificate 上传/粘贴/路径写回 web_cert_file / web_key_file。
func (s *ServerConfigController) SaveWebCertificate() {
	s.saveScopedCertificate("web", "面板", "web_cert_file", "web_key_file", "server_config.web_certificate.update")
}

// SaveBridgeCertificate 上传/粘贴/路径写回 bridge_cert_file / bridge_key_file。
func (s *ServerConfigController) SaveBridgeCertificate() {
	s.saveScopedCertificate("bridge", "Bridge", "bridge_cert_file", "bridge_key_file", "server_config.bridge_certificate.update")
}
