package controllers

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"ehang.io/nps/web/audit"
	"github.com/astaxie/beego"
)

// ServerConfigController nps.conf 分组编辑（白名单写回 + 备份 + 重启提示）。
type ServerConfigController struct {
	BaseController
}

var confSensitiveKeys = map[string]struct{}{
	"auth_key": {}, "auth_crypt_key": {}, "web_password": {},
	"password": {}, "public_vkey": {}, "vkey": {},
	"ssl_zerossl_api": {}, "totp_secret": {}, "web_secure_entry": {}, "smtp_pass": {},
	"notify_wecom_webhook": {}, "notify_telegram_bot_token": {},
	"notify_webhook_url": {}, "notify_webhook_header": {},
}

func (s *ServerConfigController) Prepare() {
	s.BaseController.Prepare()
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
	}
}

func (s *ServerConfigController) Index() {
	s.Data["menu"] = "serverconfig"
	s.SetInfo("server config")
	confPath := serverConfigPath()
	raw, err := os.ReadFile(confPath)
	content := ""
	msg := ""
	sections := []confSectionView{}
	if err != nil {
		msg = "无法读取 conf/nps.conf：" + err.Error()
	} else {
		content = string(raw)
		sections = buildConfSections(content)
	}
	s.Data["conf_path"] = confPath
	s.Data["conf_content"] = maskConfContent(content)
	s.Data["conf_msg"] = msg
	s.Data["conf_sections"] = sections
	s.Data["audit_log_path"] = beego.AppConfig.String("audit_log_path")
	s.Data["audit_enabled"] = audit.Path() != ""
	s.Data["audit_connection"] = beego.AppConfig.DefaultBool("audit_connection", false)
	s.Data["bridge_secure_handshake"] = beego.AppConfig.DefaultBool("bridge_secure_handshake", false)
	s.Data["bridge_require_tls"] = beego.AppConfig.String("bridge_require_tls")
	s.Data["edit_hint"] = "仅白名单键可写；敏感项留空表示不改；保存前自动备份为 nps.conf.bak.*；改动需重启 nps 生效。证书可上传/粘贴托管到 conf/webssl 或 conf/bridgessl。"
	values := confKeyValues(content)
	s.Data["web_cert_status"] = buildWebCertStatus(values)
	s.Data["bridge_cert_status"] = buildBridgeCertStatus(values)
	s.Data["acme_status"] = buildAcmeStatus(values)
	s.display("serverconfig/index")
}

// Save POST 写回白名单键；返回 restart_required。
func (s *ServerConfigController) Save() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	path := serverConfigPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		s.AjaxErr("读取 nps.conf 失败: " + err.Error())
		return
	}
	updates, err := parseConfUpdates(s.Ctx.Request, string(raw))
	if err != nil {
		s.auditPanel("server_config.update", "nps.conf", 0, 0, "failure", err.Error(), nil)
		s.AjaxErr(err.Error())
		return
	}
	changed, err := writeServerConfigUpdates(path, updates)
	if err != nil {
		s.auditPanel("server_config.update", "nps.conf", 0, 0, "failure", err.Error(), nil)
		s.AjaxErr("保存 nps.conf 失败: " + err.Error())
		return
	}
	details := map[string]string{"changed": strconv.Itoa(len(changed))}
	if len(changed) > 0 {
		details["keys"] = joinKeysLimited(changed, 12)
	}
	s.auditPanel("server_config.update", "nps.conf", 0, 0, "success", "", details)
	msg := "配置未变更"
	if len(changed) > 0 {
		msg = "配置已保存，需要重启 NPS 才会生效"
	}
	s.Data["json"] = map[string]interface{}{
		"status":           1,
		"msg":              msg,
		"changed":          changed,
		"restart_required": len(changed) > 0,
	}
	s.ServeJSON()
	s.StopRun()
}

func joinKeysLimited(keys []string, max int) string {
	if len(keys) <= max {
		return strings.Join(keys, ",")
	}
	return strings.Join(keys[:max], ",") + ",..."
}

func maskConfContent(raw string) string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") || strings.HasPrefix(trim, ";") {
			out = append(out, line)
			continue
		}
		eq := strings.IndexByte(trim, '=')
		if eq <= 0 {
			out = append(out, line)
			continue
		}
		key := strings.TrimSpace(trim[:eq])
		if _, ok := confSensitiveKeys[strings.ToLower(key)]; !ok {
			out = append(out, line)
			continue
		}
		idx := strings.Index(line, key)
		if idx < 0 {
			out = append(out, key+"=******")
			continue
		}
		out = append(out, line[:idx]+key+"=******")
	}
	return strings.Join(out, "\n")
}
