package client

import (
	"os"
	"runtime"
	"strings"
)

// shellHome 面板终端家目录；拒绝 "/" 以免误伤。
func shellHome() string {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" && home != "/" {
		return home
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" {
		return home
	}
	return "."
}

// buildShellEnv 在 base 上构造面板 shell 环境。
func buildShellEnv(base []string, home, hostname string) []string {
	filtered := make([]string, 0, len(base)+8)
	skip := map[string]struct{}{
		"HOME": {}, "PS1": {}, "TERM": {}, "HOSTNAME": {}, "USER": {}, "LOGNAME": {},
	}
	for _, e := range base {
		key, _, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if _, drop := skip[key]; drop {
			continue
		}
		filtered = append(filtered, e)
	}
	filtered = append(filtered,
		"HOME="+home,
		"TERM=xterm-256color",
		"HOSTNAME="+hostname,
		"PS1=\\u@\\h:\\w\\$ ",
	)
	if runtime.GOOS != "windows" {
		filtered = append(filtered, "USER=root", "LOGNAME=root")
	}
	return filtered
}
