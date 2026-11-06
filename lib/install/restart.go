package install

import (
	"log"
	"os"
	"os/exec"
	"time"

	"ehang.io/nps/lib/common"
	"github.com/kardianos/service"
)

// RestartServicesHook 由 cmd/npc 注入：更新成功后先重启已注册的多 vkey 系统服务。
// 未设置时跳过（GUI / 库调用场景）。
var RestartServicesHook func()

// RestartClientServices 仅重启已注册客户端服务（供 `npc update` 等 CLI；不 Exit、不 relaunch）。
func RestartClientServices() {
	if RestartServicesHook == nil {
		log.Println("更新完成；未注册 RestartServicesHook，请手动重启客户端服务")
		return
	}
	RestartServicesHook()
	log.Println("已尝试重启已注册的 npc 系统服务")
}

// AfterClientUpdate 二进制已替换后的统一收尾（菜单更新与面板远程升级共用）：
// 1) 可选 RestartServicesHook（对已运行的 nps-client-* 做 systemctl/服务 restart）；
// 2) 交互/前台：relaunchSelf 后再 Exit，避免无 systemd 托管时更新后无进程；
// 3) 系统服务（非 Interactive）：禁止 relaunch，只 Exit(0)，由 systemd Restart=always 拉起，
//    避免与 unit 各起一份造成双开。Linux CLI 无托盘拦截，Exit 一定结束旧进程。
func AfterClientUpdate() {
	RestartClientServices()
	if service.Interactive() {
		relaunchSelf()
	}
	time.Sleep(time.Second)
	os.Exit(0)
}

func relaunchSelf() {
	exe := common.GetExecutablePath()
	args := os.Args[1:]
	cmd := exec.Command(exe, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		log.Println("拉起新进程失败：", err)
		return
	}
	log.Println("已拉起新进程 pid=", cmd.Process.Pid)
}
