package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
)

// confField 可编辑 nps.conf 字段元数据（白名单；未登记键不可写）。
type confField struct {
	Key         string
	Label       string
	Category    string
	Description string
	Type        string // text|bool|int|select
	Options     []string
	Optional    bool
	Sensitive   bool
}

type confFieldView struct {
	Key          string
	Label        string
	Category     string
	Description  string
	Type         string
	Options      []string
	Optional     bool
	Sensitive    bool
	Present      bool
	Value        string
	DisplayValue string
	FileStatus   string // exists|missing|空（非路径字段）
}

type confSectionView struct {
	Name   string
	Anchor string // 页面内锚点，仅字母数字与连字符
	Fields []confFieldView
}

func confSectionAnchor(name string, index int) string {
	var b strings.Builder
	b.WriteString("sec-")
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '/' || r == '(' || r == ')':
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	if s == "sec" || s == "sec-" {
		return fmt.Sprintf("sec-%d", index+1)
	}
	return s
}

type confUpdate struct {
	Key   string
	Value string
	Omit  bool
}

// confEditableCatalog 融合工作区当前支持的可写键（仅增不默认开危险项）。
var confEditableCatalog = []confField{
	{Key: "http_proxy_ip", Label: "HTTP/HTTPS 监听地址", Category: "HTTP(S) 入口", Description: "域名反代监听地址"},
	{Key: "http_proxy_port", Label: "HTTP 端口", Category: "HTTP(S) 入口", Description: "留空可关闭", Optional: true},
	{Key: "https_proxy_port", Label: "HTTPS 端口", Category: "HTTP(S) 入口", Description: "留空可关闭", Optional: true},
	{Key: "http3_proxy_port", Label: "HTTP/3 反代端口", Category: "HTTP(S) 入口", Description: "UDP；需同时启用 HTTPS；0/空=不监听", Optional: true, Type: "int"},
	{Key: "trusted_proxy_ips", Label: "可信代理 IP（兼容）", Category: "运行与安全", Description: "旧配置兼容字段；当前不参与取 IP。allow_x_real_ip / 远程升级 / WAF 开关已迁至「全局参数」热更新", Optional: true},

	{Key: "waf_rules_path", Label: "WAF 规则路径", Category: "WAF", Description: "默认 conf/waf.json；规则内容可热加载；功能开关在「全局参数」", Optional: true},
	{Key: "geoip_path", Label: "GeoIP 数据路径", Category: "WAF", Description: "默认 conf/geoip.dat；改路径需重启；二进制勿提交 git", Optional: true},
	{Key: "geosite_path", Label: "Geosite 数据路径", Category: "WAF", Description: "可选；默认 conf/geosite.dat；改路径需重启", Optional: true},

	{Key: "bridge_type", Label: "主 Bridge 协议", Category: "Bridge", Description: "主传输协议", Type: "select", Options: []string{"tcp", "kcp"}},
	{Key: "bridge_port", Label: "Bridge 端口", Category: "Bridge", Description: "npc -server 主端口", Type: "int"},
	{Key: "bridge_ip", Label: "Bridge 监听地址", Category: "Bridge", Description: "本机监听地址"},
	{Key: "bridge_kcp_port", Label: "KCP 旁路端口", Category: "Bridge", Description: "未配置则不监听", Optional: true, Type: "int"},
	{Key: "bridge_ws_port", Label: "WS 旁路端口", Category: "Bridge", Description: "未配置则不监听", Optional: true, Type: "int"},
	{Key: "bridge_wss_port", Label: "WSS 旁路端口", Category: "Bridge", Description: "未配置则不监听", Optional: true, Type: "int"},
	{Key: "bridge_quic_port", Label: "QUIC 旁路端口", Category: "Bridge", Description: "UDP；npc -type=quic；未配置则不监听", Optional: true, Type: "int"},
	{Key: "bridge_path", Label: "WebSocket 路径", Category: "Bridge", Description: "须以 / 开头", Optional: true},
	{Key: "bridge_select_mode", Label: "同密钥选主", Category: "Bridge", Description: "primary/roundrobin/random", Optional: true},
	{Key: "bridge_require_tls", Label: "强制 TLS Bridge", Category: "Bridge", Description: "默认关；开启会拒绝旧明文 npc", Type: "bool", Optional: true},
	{Key: "bridge_secure_handshake", Label: "安全握手", Category: "Bridge", Description: "默认关；旧 npc 需保持关闭", Type: "bool", Optional: true},
	{Key: "tls_enable", Label: "启用 TLS Bridge", Category: "Bridge", Description: "是否开启 tls_bridge_port", Type: "bool", Optional: true},
	{Key: "tls_bridge_port", Label: "TLS Bridge 端口", Category: "Bridge", Description: "TLS 控制端口", Type: "int", Optional: true},

	{Key: "public_vkey", Label: "公共 vkey", Category: "认证", Description: "留空关闭公共密钥", Optional: true, Sensitive: true},
	{Key: "auth_key", Label: "Web API auth_key", Category: "认证", Description: "签名 API 密钥", Sensitive: true},
	{Key: "auth_crypt_key", Label: "auth_crypt_key", Category: "认证", Description: "必须恰好 16 字符", Sensitive: true},
	{Key: "auth_time_window", Label: "握手时间窗(秒)", Category: "认证", Description: "挑战握手时间偏差", Type: "int", Optional: true},

	{Key: "web_host", Label: "面板域名", Category: "Web 面板", Description: "端口复用时分流用", Optional: true},
	{Key: "web_username", Label: "管理员用户名", Category: "Web 面板", Description: "面板登录名"},
	{Key: "web_password", Label: "管理员密码", Category: "Web 面板", Description: "留空表示不修改", Sensitive: true},
	{Key: "web_port", Label: "面板端口", Category: "Web 面板", Description: "管理面板监听端口", Type: "int"},
	{Key: "web_ip", Label: "面板监听地址", Category: "Web 面板", Description: "本机监听地址"},
	{Key: "web_base_url", Label: "URL 前缀", Category: "Web 面板", Description: "反代子路径如 /nps", Optional: true},
	{Key: "web_open_ssl", Label: "面板 HTTPS", Category: "Web 面板", Description: "是否启用面板 TLS", Type: "bool"},
	{Key: "totp_secret", Label: "管理员 TOTP 密钥", Category: "Web 面板", Description: "空=关闭 2FA；可用 nps -gen2fa 生成；留空保存表示不修改；保存后需重启", Optional: true, Sensitive: true},
	{Key: "web_secure_entry", Label: "安全入口路径", Category: "Web 面板", Description: "空=关闭；如 /s/随机串；须先访问该路径才显示登录页；留空保存表示不修改；保存后需重启", Optional: true, Sensitive: true},
	{Key: "web_cert_file", Label: "面板证书路径", Category: "证书路径", Description: "web_open_ssl=true 时必填；也可用下方「证书托管」上传/粘贴", Optional: true},
	{Key: "web_key_file", Label: "面板私钥路径", Category: "证书路径", Description: "与证书成对", Optional: true},
	{Key: "bridge_cert_file", Label: "Bridge 证书路径", Category: "证书路径", Description: "可选；未配置则使用临时自签名", Optional: true},
	{Key: "bridge_key_file", Label: "Bridge 私钥路径", Category: "证书路径", Description: "与 Bridge 证书成对", Optional: true},
	{Key: "ssl_email", Label: "ACME 邮箱", Category: "ACME 自动证书", Description: "联系邮箱；空则仍可申请但建议填写", Optional: true},
	{Key: "ssl_path", Label: "ACME 缓存目录", Category: "ACME 自动证书", Description: "默认 conf/ssl", Optional: true},
	{Key: "force_auto_ssl", Label: "强制 ACME（非 80/443）", Category: "ACME 自动证书", Description: "默认关；非标准端口须开启才申请", Type: "bool", Optional: true},
	{Key: "ssl_ca", Label: "ACME CA", Category: "ACME 自动证书", Description: "空/letsencrypt=Let's Encrypt；zerossl=ZeroSSL", Type: "select", Options: []string{"letsencrypt", "zerossl"}, Optional: true},
	{Key: "ssl_zerossl_api", Label: "ZeroSSL API Key", Category: "ACME 自动证书", Description: "ssl_ca=zerossl 时必填；留空表示不修改", Optional: true, Sensitive: true},

	{Key: "flow_store_interval", Label: "流量持久化(分)", Category: "日志与审计", Description: "0/空表示不持久化", Optional: true, Type: "int"},
	{Key: "log_level", Label: "日志级别", Category: "日志与审计", Description: "0-7"},
	{Key: "log_path", Label: "日志路径", Category: "日志与审计", Description: "主日志文件", Optional: true},
	{Key: "audit_log_path", Label: "审计日志路径", Category: "日志与审计", Description: "空=关闭管理面审计", Optional: true},
	{Key: "audit_connection", Label: "连接审计", Category: "日志与审计", Description: "需同时配置 audit_log_path", Type: "bool", Optional: true},
	{Key: "audit_connection_min_interval", Label: "连接审计去抖(秒)", Category: "日志与审计", Description: "0=不去抖；默认 30", Type: "int", Optional: true},
	{Key: "disconnect_timeout", Label: "客户端断线超时", Category: "日志与审计", Description: "秒", Type: "int", Optional: true},

	{Key: "p2p_ip", Label: "P2P 地址", Category: "P2P", Description: "中继监听地址", Optional: true},
	{Key: "p2p_port", Label: "P2P 端口", Category: "P2P", Description: "0/空关闭", Optional: true, Type: "int"},
	{Key: "p2p_mode", Label: "P2P 模式提示", Category: "P2P", Description: "legacy/stun/modern；由 npc -p2p_mode 启用", Optional: true, Type: "select", Options: []string{"legacy", "stun", "modern"}},
	{Key: "p2p_stun_servers", Label: "STUN 服务器", Category: "P2P", Description: "供文档/运维参考；npc 用 -stun_addr", Optional: true},

	{Key: "http_error_page_path", Label: "自定义错误页路径", Category: "错误页与通知", Description: "相对运行目录或绝对路径；空=内置页；改路径需重启。品牌名/通知通道已迁至「全局参数」热更新", Optional: true},
}

func serverConfigPath() string {
	return filepath.Join(common.GetRunPath(), "conf", "nps.conf")
}

func confKeyValues(content string) map[string]string {
	values := make(map[string]string)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" || strings.ContainsAny(key, " \t[]") {
			continue
		}
		values[key] = strings.TrimSpace(parts[1])
	}
	return values
}

func confKeyFromLine(raw string) (string, bool) {
	line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return "", false
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" || strings.ContainsAny(key, " \t[]") {
		return "", false
	}
	return key, true
}

func buildConfSections(content string) []confSectionView {
	current := confKeyValues(content)
	order := make([]string, 0)
	byCat := make(map[string][]confFieldView)
	for _, meta := range confEditableCatalog {
		val, present := current[meta.Key]
		display := val
		if meta.Sensitive && present && val != "" {
			display = "******"
		}
		fv := confFieldView{
			Key: meta.Key, Label: meta.Label, Category: meta.Category,
			Description: meta.Description, Type: meta.Type, Options: meta.Options,
			Optional: meta.Optional, Sensitive: meta.Sensitive,
			Present: present, Value: val, DisplayValue: display,
		}
		if meta.Type == "" {
			fv.Type = "text"
		}
		if strings.HasSuffix(meta.Key, "_cert_file") || strings.HasSuffix(meta.Key, "_key_file") {
			fv.FileStatus = "missing"
			if present && val != "" {
				p := common.ResolveConfigPath(val)
				if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
					fv.FileStatus = "exists"
				}
			}
		}
		if _, ok := byCat[meta.Category]; !ok {
			order = append(order, meta.Category)
		}
		byCat[meta.Category] = append(byCat[meta.Category], fv)
	}
	out := make([]confSectionView, 0, len(order))
	for i, name := range order {
		out = append(out, confSectionView{
			Name:   name,
			Anchor: confSectionAnchor(name, i),
			Fields: byCat[name],
		})
	}
	return out
}

func validateConfValue(meta confField, value string) error {
	if value == "" {
		if meta.Optional || meta.Sensitive {
			return nil
		}
		return fmt.Errorf("配置 %s 不能为空", meta.Key)
	}
	switch meta.Type {
	case "bool":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("配置 %s 必须是 true 或 false", meta.Key)
		}
	case "int":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("配置 %s 必须是非负整数", meta.Key)
		}
		if strings.Contains(meta.Key, "port") && n > 65535 {
			return fmt.Errorf("配置 %s 端口范围必须是 0-65535", meta.Key)
		}
	case "select":
		ok := false
		for _, opt := range meta.Options {
			if value == opt {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("配置 %s 的值无效", meta.Key)
		}
	}
	if meta.Key == "auth_crypt_key" && len(value) != 16 {
		return errors.New("auth_crypt_key 必须恰好为 16 个字符")
	}
	if meta.Key == "bridge_path" && !strings.HasPrefix(value, "/") {
		return errors.New("bridge_path 必须以 / 开头")
	}
	if meta.Key == "web_base_url" && value != "" && !strings.HasPrefix(value, "/") {
		return errors.New("web_base_url 必须为空或以 / 开头")
	}
	if meta.Key == "totp_secret" && value != "" && !crypt.IsValidTOTPSecret(value) {
		return errors.New("totp_secret 不是有效的 base32 TOTP 密钥（可用 nps -gen2fa 生成）")
	}
	if meta.Key == "web_secure_entry" && value != "" {
		if _, err := NormalizeWebSecureEntry(value); err != nil {
			return err
		}
	}
	if meta.Key == "log_level" {
		level := strings.ToLower(strings.TrimSpace(value))
		if n, err := strconv.Atoi(level); err == nil {
			if n < 0 || n > 7 {
				return errors.New("log_level 必须是 0-7")
			}
		}
	}
	return nil
}

func parseConfUpdates(request *http.Request, content string) ([]confUpdate, error) {
	if request == nil {
		return nil, errors.New("请求无效")
	}
	if err := request.ParseForm(); err != nil {
		return nil, fmt.Errorf("读取表单失败: %w", err)
	}
	current := confKeyValues(content)
	updates := make([]confUpdate, 0)
	for _, meta := range confEditableCatalog {
		raw := strings.TrimSpace(request.FormValue("config_" + meta.Key))
		if strings.ContainsAny(raw, "\r\n") {
			return nil, fmt.Errorf("配置 %s 不能包含换行", meta.Key)
		}
		enabled := true
		if meta.Optional {
			enabled = request.FormValue("config_enabled_"+meta.Key) != ""
		}
		if meta.Optional && !enabled {
			if _, present := current[meta.Key]; present {
				updates = append(updates, confUpdate{Key: meta.Key, Omit: true})
			}
			continue
		}
		if meta.Sensitive && raw == "" {
			// 脱敏字段留空 = 保持原值
			continue
		}
		if err := validateConfValue(meta, raw); err != nil {
			return nil, err
		}
		if raw == "" && meta.Optional {
			if _, present := current[meta.Key]; present {
				updates = append(updates, confUpdate{Key: meta.Key, Omit: true})
			}
			continue
		}
		updates = append(updates, confUpdate{Key: meta.Key, Value: raw})
	}
	if err := validateConfUpdateRelations(current, updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func effectiveConfValues(current map[string]string, updates []confUpdate) map[string]string {
	values := make(map[string]string, len(current)+len(updates))
	for k, v := range current {
		values[k] = v
	}
	for _, u := range updates {
		if u.Omit {
			delete(values, u.Key)
			continue
		}
		values[u.Key] = u.Value
	}
	return values
}

func validateConfUpdateRelations(current map[string]string, updates []confUpdate) error {
	values := effectiveConfValues(current, updates)
	if strings.EqualFold(strings.TrimSpace(values["web_open_ssl"]), "true") {
		if strings.TrimSpace(values["web_cert_file"]) == "" || strings.TrimSpace(values["web_key_file"]) == "" {
			return errors.New("启用面板 HTTPS 时，证书路径和私钥路径必须同时填写")
		}
	}
	webCert := strings.TrimSpace(values["web_cert_file"])
	webKey := strings.TrimSpace(values["web_key_file"])
	if (webCert == "") != (webKey == "") {
		return errors.New("面板证书路径和私钥路径必须同时填写或同时留空")
	}
	bridgeCert := strings.TrimSpace(values["bridge_cert_file"])
	bridgeKey := strings.TrimSpace(values["bridge_key_file"])
	if (bridgeCert == "") != (bridgeKey == "") {
		return errors.New("Bridge 证书路径和私钥路径必须同时填写或同时留空")
	}
	ca := strings.ToLower(strings.TrimSpace(values["ssl_ca"]))
	if ca == "zerossl" || ca == "zero_ssl" || ca == "zero ssl" {
		if strings.TrimSpace(values["ssl_zerossl_api"]) == "" {
			return errors.New("ssl_ca=zerossl 时必须配置 ssl_zerossl_api")
		}
	}
	return nil
}

func uniqueConfKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func backupServerConfig(path string) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	bak := path + ".bak." + time.Now().Format("20060102150405")
	dst, err := os.OpenFile(bak, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return bak, nil
}

func writeConfAtomically(path string, data []byte) error {
	target := path
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		target, err = filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("解析配置软链接失败: %w", err)
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("配置不是普通文件: %s", target)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".nps.conf.web-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

// writeServerConfigUpdates 按白名单更新；保留注释与未知键；返回变更键列表。
func writeServerConfigUpdates(path string, updates []confUpdate) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(updates) == 0 {
		return nil, nil
	}
	updateByKey := make(map[string]confUpdate, len(updates))
	for _, u := range updates {
		updateByKey[u.Key] = u
	}
	seen := make(map[string]struct{})
	changed := make([]string, 0)
	lines := strings.SplitAfter(string(data), "\n")
	result := make([]string, 0, len(lines)+len(updates)+2)
	for _, line := range lines {
		key, ok := confKeyFromLine(line)
		u, has := updateByKey[key]
		if !ok || !has {
			result = append(result, line)
			continue
		}
		seen[key] = struct{}{}
		if u.Omit {
			changed = append(changed, key)
			continue
		}
		newline := ""
		if strings.HasSuffix(line, "\n") {
			newline = "\n"
		}
		replacement := key + "=" + u.Value + newline
		if strings.TrimSpace(strings.TrimSuffix(line, "\n")) != strings.TrimSpace(strings.TrimSuffix(replacement, "\n")) {
			changed = append(changed, key)
		}
		result = append(result, replacement)
	}
	missing := make([]string, 0)
	for _, u := range updates {
		if _, exists := seen[u.Key]; exists || u.Omit {
			continue
		}
		missing = append(missing, u.Key+"="+u.Value)
		changed = append(changed, u.Key)
	}
	if len(missing) > 0 {
		text := strings.Join(result, "")
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n# Maintained by the NPS Web configuration panel.\n" + strings.Join(missing, "\n") + "\n"
		result = []string{text}
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if _, err := backupServerConfig(path); err != nil {
		return nil, fmt.Errorf("备份 nps.conf 失败: %w", err)
	}
	if err := writeConfAtomically(path, []byte(strings.Join(result, ""))); err != nil {
		return nil, err
	}
	return uniqueConfKeys(changed), nil
}
