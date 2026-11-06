package install

import (
	"path/filepath"
	"strings"

	"ehang.io/nps/lib/common"
)

const (
	ChannelCLI = "cli"
	ChannelGUI = "gui"
)

// UpdateResult 客户端自更新结果（菜单 / 远程升级共用）。
type UpdateResult struct {
	Updated bool
	Skipped bool // 已是最新，未改文件
	Channel string
	Latest  string
	Message string
	Err     error
}

// ClientUpdateChannel 按当前可执行文件名判断取 cli 包还是 gui 包。
func ClientUpdateChannel() string {
	base := strings.ToLower(filepath.Base(common.GetExecutablePath()))
	base = strings.TrimSuffix(base, ".exe")
	if strings.Contains(base, "npc-gui") {
		return ChannelGUI
	}
	return ChannelCLI
}
