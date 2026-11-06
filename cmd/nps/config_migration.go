package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// nps.conf 只增迁移。
// 对已有配置：补齐模板里有、现网文件没有的键；保留注释、未知键、显式空值。
// 不对存量部署静默轮换密码/密钥（见 docs/upgrade-compatibility.md）。

type configCompatibilityRule struct {
	canonical string
	aliases   []string
}

var configCompatibilityAliases = []configCompatibilityRule{
	{canonical: "bridge_port", aliases: []string{"bridge_tcp_port", "bridge_kcp_port"}},
	{canonical: "tls_bridge_port", aliases: []string{"bridge_tls_port"}},
	{canonical: "bridge_ip", aliases: []string{"bridge_tcp_ip", "bridge_kcp_ip"}},
}

// 可选键：运维可故意从 conf 删掉以关闭功能；升级时不要再塞回去。
var configMigrationOptionalKeys = map[string]struct{}{
	"http_proxy_port": {}, "https_proxy_port": {},
	"public_vkey": {}, "p2p_ip": {}, "p2p_port": {},
	"web_host": {}, "web_base_url": {},
	"web_cert_file": {}, "web_key_file": {},
	"bridge_cert_file": {}, "bridge_key_file": {},
	"ssl_email": {}, "ssl_path": {}, "force_auto_ssl": {},
	"flow_store_interval": {}, "log_path": {},
	"http_cache_length": {}, "pprof_ip": {}, "pprof_port": {},
	"tls_bridge_port": {},
	// 可选旁路：运维未配置端口则不回填，避免升级后突然多开端口
	"bridge_kcp_port": {}, "bridge_ws_port": {}, "bridge_wss_port": {},
	"bridge_quic_port": {}, "bridge_path": {},
	"bridge_select_mode": {}, "bridge_require_tls": {},
	"bridge_secure_handshake": {}, "auth_time_window": {},
	"audit_log_path": {},
	"audit_connection": {},
	"audit_connection_min_interval": {},
	"http3_proxy_port": {},
	"allow_x_real_ip": {}, "trusted_proxy_ips": {},
	"allow_remote_update": {}, "x_nps_http_only": {},
	"https_default_cert_file": {}, "https_default_key_file": {},
	"p2p_mode": {}, "p2p_stun_servers": {},
	"register_rate_limit": {},
	"smtp_host": {}, "smtp_port": {}, "smtp_user": {}, "smtp_pass": {}, "smtp_from": {},
	"smtp_security": {}, "smtp_username": {}, "smtp_password": {},
	"register_email_verify": {}, "register_email_code_ttl": {}, "register_email_code_cooldown": {},
	"allow_web_terminal": {},
	"release_api_url": {}, "release_download_base": {},
	"web_welcome": {}, "web_subtitle": {},
	"totp_secret": {}, "web_secure_entry": {},
}

func migrateConfigFile(path, template string) (changed bool, added []string, err error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, nil, err
	}
	original := string(content)
	compatibility, compatibilityAdded := migrateConfigCompatibility(original)
	merged, templateAdded := mergeMissingConfigFields(compatibility, template)
	added = append(compatibilityAdded, templateAdded...)
	if merged == original {
		return false, nil, nil
	}
	if err := writeConfigAtomically(path, []byte(merged)); err != nil {
		return false, added, err
	}
	return true, added, nil
}

func migrateConfigCompatibility(content string) (string, []string) {
	values := configValues(content)
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}
	bridgeType := strings.ToLower(strings.TrimSpace(values["bridge_type"]))
	_, bridgeTypeSet := values["bridge_type"]

	var blocks []string
	var added []string
	for _, rule := range configCompatibilityAliases {
		if _, exists := values[rule.canonical]; exists {
			continue
		}
		aliases := preferredConfigAliases(rule, bridgeType, bridgeTypeSet, values)
		for _, alias := range aliases {
			value, exists := values[alias]
			if !exists {
				continue
			}
			blocks = append(blocks,
				"# Added automatically for Bridge listener compatibility (alias: "+alias+")."+newline+
					rule.canonical+"="+value,
			)
			values[rule.canonical] = value
			added = append(added, rule.canonical)
			break
		}
	}
	if len(blocks) == 0 {
		return content, nil
	}
	return appendConfigBlocks(content, blocks, newline), added
}

func preferredConfigAliases(rule configCompatibilityRule, bridgeType string, bridgeTypeSet bool, values map[string]string) []string {
	if rule.canonical == "bridge_port" {
		switch bridgeType {
		case "kcp":
			return []string{"bridge_kcp_port", "bridge_tcp_port"}
		case "tcp":
			return []string{"bridge_tcp_port"}
		default:
			if _, hasTCP := values["bridge_tcp_port"]; hasTCP {
				return []string{"bridge_tcp_port", "bridge_kcp_port"}
			}
			if !bridgeTypeSet {
				return []string{"bridge_kcp_port", "bridge_tcp_port"}
			}
			return []string{"bridge_tcp_port"}
		}
	}
	if rule.canonical == "bridge_ip" {
		switch bridgeType {
		case "kcp":
			return []string{"bridge_kcp_ip", "bridge_tcp_ip"}
		case "tcp":
			return []string{"bridge_tcp_ip"}
		default:
			if _, hasTCP := values["bridge_tcp_ip"]; hasTCP {
				return []string{"bridge_tcp_ip", "bridge_kcp_ip"}
			}
			if !bridgeTypeSet {
				return []string{"bridge_kcp_ip", "bridge_tcp_ip"}
			}
			return []string{"bridge_tcp_ip"}
		}
	}
	return append([]string(nil), rule.aliases...)
}

func mergeMissingConfigFields(content, template string) (string, []string) {
	existing := configKeys(content)
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}

	var blocks []string
	var added []string
	var pending []string
	for _, rawLine := range strings.Split(template, "\n") {
		line := strings.TrimSuffix(rawLine, "\r")
		if key, ok := configEntryKey(line); ok {
			if _, optional := configMigrationOptionalKeys[key]; optional {
				pending = pending[:0]
				continue
			}
			if _, ok := existing[key]; !ok {
				start := 0
				for start < len(pending) && strings.TrimSpace(pending[start]) == "" {
					start++
				}
				blockLines := append(append([]string(nil), pending[start:]...), line)
				block := strings.TrimRight(strings.Join(blockLines, newline), "\r\n")
				if block != "" {
					blocks = append(blocks, block)
				}
				added = append(added, key)
				existing[key] = struct{}{}
			}
			pending = pending[:0]
			continue
		}
		pending = append(pending, line)
	}
	if len(added) == 0 {
		return content, nil
	}
	blocks = append([]string{"# The following settings were added automatically by this NPS release."}, blocks...)
	return appendConfigBlocks(content, blocks, newline), added
}

func configValues(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		separator := strings.IndexByte(line, '=')
		if separator < 0 {
			continue
		}
		key := strings.TrimSpace(line[:separator])
		if key == "" || strings.ContainsAny(key, " \t[]") {
			continue
		}
		values[key] = strings.TrimSpace(line[separator+1:])
	}
	return values
}

func configKeys(content string) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, line := range strings.Split(content, "\n") {
		if key, ok := configEntryKey(line); ok {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func configEntryKey(line string) (string, bool) {
	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	line = strings.TrimPrefix(line, "\ufeff")
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return "", false
	}
	separator := strings.IndexByte(line, '=')
	if separator < 0 {
		return "", false
	}
	key := strings.TrimSpace(line[:separator])
	if key == "" || strings.ContainsAny(key, " \t[]") {
		return "", false
	}
	return key, true
}

func appendConfigBlocks(content string, blocks []string, newline string) string {
	merged := content
	if merged != "" {
		if !strings.HasSuffix(merged, "\n") && !strings.HasSuffix(merged, "\r") {
			merged += newline
		}
		merged += newline
	}
	merged += strings.Join(blocks, newline+newline) + newline
	return merged
}

func writeConfigAtomically(path string, data []byte) error {
	target := path
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		target, err = filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve config symlink: %w", err)
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("config target is not a regular file: %s", target)
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary config permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
