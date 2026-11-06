package controllers

import "strings"

// formFlagReader 供 mixProxyFlagsFromForm 读取表单；签名对齐 beego.Controller.GetString。
type formFlagReader interface {
	GetString(key string, def ...string) string
	GetBoolNoErr(key string, def ...bool) bool
}

// mixProxyFlagsFromForm 读取混合代理开关；非 mixProxy 返回 false,false。
// mixProxy 且表单未提交开关时默认两者都开。
func mixProxyFlagsFromForm(s formFlagReader, mode string) (httpProxy, socks5Proxy bool) {
	if strings.TrimSpace(mode) != "mixProxy" {
		return false, false
	}
	if s.GetString("http_proxy") == "" && s.GetString("socks5_proxy") == "" {
		return true, true
	}
	return s.GetBoolNoErr("http_proxy", true), s.GetBoolNoErr("socks5_proxy", true)
}

// allowedTunnelModes 面板/API 允许创建的隧道模式。
// 禁止写入内部 mode（如 webServer），避免启动递归崩溃。
var allowedTunnelModes = map[string]struct{}{
	"tcp":         {},
	"udp":         {},
	"httpProxy":   {},
	"socks5":      {},
	"mixProxy":    {},
	"secret":      {},
	"p2p":         {},
	"file":        {},
	"shadowsocks": {},
}

// IsAllowedTunnelMode 供控制器校验隧道类型。
func IsAllowedTunnelMode(mode string) bool {
	_, ok := allowedTunnelModes[strings.TrimSpace(mode)]
	return ok
}

// validateClientWebPair 校验客户端 Web 登录用户名和密码必须成对设置。
func validateClientWebPair(webUserName, webPassword string) string {
	hasUserName := strings.TrimSpace(webUserName) != ""
	hasPassword := strings.TrimSpace(webPassword) != ""
	if hasUserName != hasPassword {
		return "web username and password must be set together"
	}
	return ""
}

// validateTunnelInput 按隧道模式校验必填项，并强制 mode 白名单。
func validateTunnelInput(mode, target, password, localPath string) string {
	mode = strings.TrimSpace(mode)
	if !IsAllowedTunnelMode(mode) {
		return "invalid tunnel type"
	}
	switch mode {
	case "tcp", "udp":
		if strings.TrimSpace(target) == "" {
			return "target is required"
		}
	case "secret", "p2p":
		if strings.TrimSpace(target) == "" {
			return "target is required"
		}
		if strings.TrimSpace(password) == "" {
			return "password is required"
		}
	case "file":
		if strings.TrimSpace(localPath) == "" {
			return "local path is required"
		}
	case "socks5", "httpProxy", "mixProxy":
		// 无额外必填
	case "shadowsocks":
		// 密码可空：控制器会 NormalizeShadowsocksPSK 自动生成
	}
	return ""
}

// validateHostInput 校验域名解析必填项。
func validateHostInput(host, target string) string {
	if strings.TrimSpace(host) == "" {
		return "host is required"
	}
	if strings.TrimSpace(target) == "" {
		return "target is required"
	}
	return ""
}
