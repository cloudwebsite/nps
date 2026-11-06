package main

import (
	"bufio"
	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/daemon"
	"ehang.io/nps/server"
	"flag"
	"fmt"
	"github.com/fatih/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/lib/waf"
	"ehang.io/nps/server/connection"
	"ehang.io/nps/server/tool"
	"ehang.io/nps/web/routers"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/runtimeopts"
	"ehang.io/nps/web/audit"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"

	"github.com/kardianos/service"
)

var (
	level      string
	ver        = flag.Bool("version", false, "show current version")
	confPath   = flag.String("conf_path", "", "set current confPath")
	serverCmd  = flag.Bool("server", false, "NPS管理脚本")
	npsLogPath = flag.String("log_path", "", "nps log path")
	gen2fa     = flag.Bool("gen2fa", false, "generate a TOTP secret for Web administrator login")
	updateGeo  = flag.Bool("updategeo", false, "download latest geoip.dat/geosite.dat then exit")
)

func main() {

	debug.SetMaxThreads(1000000)

	flag.Parse()
	// init log
	if *ver {
		common.PrintVersion()
		return
	}
	if *gen2fa {
		secret, err := crypt.GenerateTOTPSecret()
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate TOTP secret failed:", err)
			os.Exit(1)
		}
		fmt.Println("TOTP secret:", secret)
		fmt.Println("Authenticator URI:", crypt.BuildTOTPURI("NPS", "admin", secret))
		fmt.Println("Write into conf/nps.conf and restart nps:")
		fmt.Println("totp_secret=" + secret)
		return
	}

	var logPath string
	// *confPath why get null value ?
	for _, v := range os.Args[1:] {
		switch v {
		case "install", "start", "stop", "uninstall", "restart":
			continue
		}
		if strings.Contains(v, "-conf_path=") {
			common.ConfPath = strings.Replace(v, "-conf_path=", "", -1)
		}

		if strings.Contains(v, "-log_path=") {
			logPath = strings.Replace(v, "-log_path=", "", -1)
		}
	}

	if *serverCmd {
		_ = logs.SetLogger(logs.AdapterConsole, `{"level":7,"color":true}`)
		printSlogan()
		inputCmd()
		return
	}

	// auto-generate default config files if not exist
	initConfig(filepath.Join(common.GetRunPath(), "conf"))

		if err := beego.LoadAppConfig("ini", filepath.Join(common.GetRunPath(), "conf", "nps.conf")); err != nil {
			log.Fatalln("load config file error", err.Error())
		}

		if *updateGeo {
			_ = logs.SetLogger(logs.AdapterConsole, `{"level":7,"color":true}`)
			ipPath, sitePath := resolveGeoPaths()
			install.SetGeoDataPaths(ipPath, sitePath)
			fmt.Println("GeoIP:", ipPath)
			fmt.Println("Geosite:", sitePath)
			result, err := install.UpdateGeoData(ipPath, sitePath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "updategeo failed:", err)
				os.Exit(1)
			}
			if result != nil && len(result.Updated) > 0 {
				fmt.Println("updated", len(result.Updated), "file(s), version", result.Version)
			} else {
				fmt.Println("already up to date, version", result.Version)
			}
			return
		}

		// 审计：空路径则关闭写盘；写失败不得阻断转发面。
			audit.Configure(beego.AppConfig.String("audit_log_path"))
			audit.SetConnectionEnabled(beego.AppConfig.DefaultBool("audit_connection", false))
			audit.SetConnectionMinInterval(beego.AppConfig.DefaultInt("audit_connection_min_interval", 30))
			if audit.Path() != "" && audit.ConnectionEnabled() {
				logs.Info("audit_connection=on：记录 npc 上下线（去抖 %v）", audit.ConnectionMinInterval())
			}
	// 首次启动/升级：nps.conf → global.json（已保存过的字段不覆盖）
	file.GetDb().MigrateGlobalFromAppConfig()
	// Geo 数据路径仍读 conf；WAF/通知/Release 源跟 global 热配置
	initGeoData()
	runtimeopts.Apply()
	if waf.Enabled() {
		panel := "面板不保护"
		if waf.ProtectPanelEnabled() {
			panel = "面板同防护"
		}
		logs.Info("WAF 已启用（%s）；规则=%s", panel, waf.RulesPath())
	}

	common.InitPProfFromFile()
	if level = beego.AppConfig.String("log_level"); level == "" {
		level = "7"
	}
	logs.Reset()
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)

	// 优先级：命令行 -log_path > 已解析参数 > nps.conf > 系统默认路径
	if logPath == "" && *npsLogPath != "" {
		logPath = *npsLogPath
	}
	if logPath == "" {
		logPath = beego.AppConfig.String("log_path")
	}
	if logPath == "" {
		logPath = common.GetLogPath()
	}
	// 相对路径按运行目录解析，避免 systemd 工作目录为 / 时落到 /nps.log
	if !filepath.IsAbs(logPath) {
		logPath = filepath.Join(common.GetRunPath(), logPath)
	}
	if common.IsWindows() {
		logPath = strings.Replace(logPath, "\\", "\\\\", -1)
	}
	*npsLogPath = logPath

	// init service
	options := make(service.KeyValue)
	svcConfig := &service.Config{
		Name:        "Nps",
		DisplayName: "nps内网穿透代理服务器",
		Description: "一款轻量级、功能强大的内网穿透代理服务器。支持tcp、udp流量转发，支持内网http代理、内网socks5代理，同时支持snappy压缩、站点保护、加密传输、多路复用、header修改等。支持web图形化管理，集成多用户模式。",
		Option:      options,
	}

	bridge.ServerTlsEnable = beego.AppConfig.DefaultBool("tls_enable", false)
	bridge.RequireTLS = bridge.ParseRequireTLS(beego.AppConfig.String("bridge_require_tls"))
	secureHS := beego.AppConfig.DefaultBool("bridge_secure_handshake", false)
	version.SetSecureHandshakeServerEnabled(secureHS)
	if secureHS {
		logs.Info("bridge_secure_handshake=on：产品版本≥%s 的 npc 走挑战握手；更旧客户端仍 legacy", version.SecureHandshakeMinVersion)
	}
	if err := bridge.SetClientSelectMode(beego.AppConfig.String("bridge_select_mode")); err != nil {
		logs.Warn("%v；回退 primary", err)
	}
	if mode := bridge.ClientSelectModeName(); mode != "primary" {
		logs.Info("bridge_select_mode=%s（同 vkey 多 WORK_CHAN 将按该策略选取；Primary 仍为后来者替换）", mode)
	}
	if api, base := install.ReleaseSourceSnapshot(); api != "" {
		logs.Info("release source api=%s download_base=%s", api, base)
	}
	if p2pMode := strings.TrimSpace(beego.AppConfig.String("p2p_mode")); p2pMode != "" && p2pMode != "legacy" {
		logs.Info("p2p_mode=%s（服务端仍提供旧 UDP 打洞；stun 辅助由 npc -p2p_mode 启用）", p2pMode)
	}

	for _, v := range os.Args[1:] {
		switch v {
		case "install", "start", "stop", "uninstall", "restart":
			continue
		}
		svcConfig.Arguments = append(svcConfig.Arguments, v)
	}

	svcConfig.Arguments = append(svcConfig.Arguments, "service")
	if len(os.Args) > 1 && os.Args[1] == "service" {
		fileCfg := `{"level":` + level + `,"filename":"` + logPath + `","daily":false,"maxlines":100000,"color":true}`
		if err := logs.SetLogger(logs.AdapterFile, fileCfg); err != nil {
			// 文件日志失败时回退控制台，避免服务模式静默无日志、排查困难
			_ = logs.SetLogger(logs.AdapterConsole, `{"level":`+level+`,"color":true}`)
			logs.Error("文件日志初始化失败（%s）：%v，已回退到控制台", logPath, err)
		}
	} else {
		_ = logs.SetLogger(logs.AdapterConsole, `{"level":`+level+`,"color":true}`)
	}
	if !common.IsWindows() {
		svcConfig.Dependencies = []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"}
		svcConfig.Option["SystemdScript"] = install.SystemdScript
		svcConfig.Option["SysvScript"] = install.SysvScript
	}
	prg := &nps{}
	prg.exit = make(chan struct{})
	s, err := service.New(prg, svcConfig)
	if err != nil {
		logs.Error(err, "service function disabled")
		run()
		// run without service
		wg := sync.WaitGroup{}
		wg.Add(1)
		wg.Wait()
		return
	}

	if len(os.Args) > 1 && os.Args[1] != "service" {
		switch os.Args[1] {
		case "reload":
			daemon.InitDaemon("nps", common.GetRunPath(), common.GetTmpPath())
			return
		case "install":
			// uninstall before
			_ = service.Control(s, "stop")
			_ = service.Control(s, "uninstall")

			binPath := install.InstallNps()
			svcConfig.Executable = binPath
			s, err := service.New(prg, svcConfig)
			if err != nil {
				logs.Error(err)
				return
			}
			err = service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				confPath := "/etc/init.d/" + svcConfig.Name
				os.Symlink(confPath, "/etc/rc.d/S90"+svcConfig.Name)
				os.Symlink(confPath, "/etc/rc.d/K02"+svcConfig.Name)
			}
			return
		case "start", "restart", "stop":
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				cmd := exec.Command("/etc/init.d/"+svcConfig.Name, os.Args[1])
				err := cmd.Run()
				if err != nil {
					logs.Error(err)
				}
				return
			}
			if err := controlNpsService(s, os.Args[1]); err != nil {
				logs.Error("%s", err.Error())
			}
			return
		case "uninstall":
			err := service.Control(s, os.Args[1])
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				os.Remove("/etc/rc.d/S90" + svcConfig.Name)
				os.Remove("/etc/rc.d/K02" + svcConfig.Name)
			}
			return
		case "update":
			if updated, err := install.UpdateNps(); err != nil {
				logs.Error("更新失败：", err)
			} else if updated {
				restartNpsService(s)
			}
			return
			//default:
			//	logs.Error("command is not support")
			//	return
		}
	}

	_ = s.Run()
}

func printSlogan() {
	confFile := ""
	for _, dir := range []string{common.GetAppPath(), common.GetRunPath()} {
		if p := filepath.Join(dir, "conf", "nps.conf"); common.FileExists(p) {
			confFile = p
			break
		}
	}
	configLoaded := false
	if confFile != "" {
		if err := beego.LoadAppConfig("ini", confFile); err == nil {
			configLoaded = true
		}
	}

	green := color.New(color.FgGreen).SprintFunc()
	w := color.Output

	fmt.Fprintf(w, "%s", green(""))

	fmt.Fprintf(w, "\033[32;0m###########################################################\n")
	fmt.Fprintf(w, "\033[32;0m#                   \033[31mNPS内网穿透服务端\033[0m                     #\n")
	fmt.Fprintf(w, "\033[32;0m#                                                         #\n")
	fmt.Fprintf(w, "\033[32;0m#\033[32m 地址：\033[31;0mhttps://github.com/cloudwebsite/nps\033[0m             #\n")
	fmt.Fprintf(w, "\033[32;0m#\033[32m 提示：\033[32;0m1、涉及到系统服务的需要以管理员/root身份运行\033[0m      #\n")
	fmt.Fprintf(w, "\033[32;0m#\033[32m       \033[32;0m2、安装/卸载/更新请选择对应的菜单项\033[0m               #\n")
	fmt.Fprintf(w, "\033[32;0m#\033[32m       \033[32;0m3、启动/停止/重启操作的是系统服务\033[0m                 #\n")
	fmt.Fprintf(w, "\033[32;0m###########################################################\n")
	fmt.Fprintf(w, "\033[0m") // 重置颜色

	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "\u001B[32m输入 [1]\u001B[0m - 安装 NPS\n")
	fmt.Fprintf(w, "\u001B[32m输入 [2]\u001B[0m - 卸载 NPS\n")
	fmt.Fprintf(w, "\u001B[32m输入 [3]\u001B[0m - 更新 NPS\n")
	fmt.Fprintf(w, "---------------------\n")
	fmt.Fprintf(w, "\u001B[32m输入 [4]\u001B[0m - 启动 NPS\n")
	fmt.Fprintf(w, "\u001B[32m输入 [5]\u001B[0m - 停止 NPS\n")
	fmt.Fprintf(w, "\u001B[32m输入 [6]\u001B[0m - 重启 NPS\n")
	fmt.Fprintf(w, "---------------------\n")
	fmt.Fprintf(w, "\u001B[32m输入 [0]\u001B[0m - 退出脚本\n")
	fmt.Fprintf(w, "---------------------\n")

	status := npsServiceStatus()
	fmt.Fprintf(w, "\u001B[32m当前版本：v%s\u001B[0m\n", version.VERSION)
	fmt.Fprintf(w, "\u001B[32m服务状态：%s\u001B[0m\n", status)
	// 管理面板地址仅当服务已启动时显示
	if status == "运行中" {
		fmt.Fprintf(w, "\u001B[32m管理面板：%s\u001B[0m\n", npsPanelAddr(configLoaded))
	}
	fmt.Fprintf(w, "\n")
}

// npsServiceStatus 返回 Nps 系统服务的运行状态描述。
func npsServiceStatus() string {
	prg := &nps{exit: make(chan struct{})}
	s, err := service.New(prg, &service.Config{Name: "Nps"})
	if err != nil {
		return "未知"
	}
	st, err := s.Status()
	if err != nil {
		return "未安装"
	}
	switch st {
	case service.StatusRunning:
		return "运行中"
	case service.StatusStopped:
		return "已停止"
	}
	return "未知"
}

// npsPanelAddr 返回管理面板的访问地址（基于当前配置）。
// configLoaded 为 false 表示未找到配置文件，无法判断面板地址。
func npsPanelAddr(configLoaded bool) string {
	if !configLoaded {
		return "未找到配置文件 conf/nps.conf"
	}
	port := strings.TrimSpace(beego.AppConfig.String("web_port"))
	if port == "" {
		return "未配置 web_port（管理面板已关闭）"
	}
	scheme := "http"
	if beego.AppConfig.DefaultBool("web_open_ssl", false) {
		scheme = "https"
	}
	host := strings.TrimSpace(beego.AppConfig.String("web_host"))
	if host == "" || host == "a.o.com" {
		host = "127.0.0.1"
	}
	base := strings.TrimSuffix(beego.AppConfig.String("web_base_url"), "/")
	return scheme + "://" + host + ":" + port + base
}

// restartNpsService 在更新完成后自动重启已安装的 Nps 系统服务，使新版本生效。
// 未安装为系统服务时不改动当前进程，仅提示手动重启。
func restartNpsService(s service.Service) {
	if _, err := s.Status(); err != nil {
		fmt.Println("未检测到已安装的 Nps 服务，请手动重启 nps 使新版本生效")
		return
	}
	if err := controlNpsService(s, "restart"); err != nil {
		fmt.Println("Nps 服务自动重启失败：", err)
		return
	}
	fmt.Println("Nps 服务已自动重启，新版本已生效")
}

// controlNpsService 控制系统服务；Windows 上 restart 改为 stop→start。
func controlNpsService(s service.Service, action string) error {
	var err error
	if action == "restart" && common.IsWindows() {
		_ = service.Control(s, "stop")
		time.Sleep(1500 * time.Millisecond)
		err = service.Control(s, "start")
	} else {
		err = service.Control(s, action)
	}
	if err == nil {
		return nil
	}
	return fmt.Errorf("%v%s", err, serviceControlHint(err))
}

func serviceControlHint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "access is denied") || strings.Contains(err.Error(), "拒绝访问") {
		return "（权限不足：Windows 请右键「以管理员身份运行」）"
	}
	// Linux：systemctl/服务未注册常见也是 exit status 5，与 Windows 拒绝访问不是一回事
	if strings.Contains(msg, "exit status 5") {
		if common.IsWindows() {
			return "（权限不足：请右键「以管理员身份运行」后再执行）"
		}
		return "（多半尚未安装系统服务：请先成功执行 nps install，再 nps start；可用 systemctl status Nps 检查）"
	}
	if strings.Contains(msg, "not installed") || strings.Contains(msg, "not found") || strings.Contains(msg, "unit") && strings.Contains(msg, "not") {
		return "（系统服务未安装：请先执行 nps install）"
	}
	return ""
}

func inputCmd() {
	var flag string
	fmt.Print("请输入[0-6]：")

	stdin := bufio.NewReader(os.Stdin)
	_, err := fmt.Fscanln(stdin, &flag)
	if err != nil {
		fmt.Println("输入有误")
	} else {
		if flag == "0" {
			os.Exit(0)
		}

		// init service

		prg := &nps{
			exit: make(chan struct{}),
		}
		options := make(service.KeyValue)
		svcConfig := &service.Config{
			Name:        "Nps",
			DisplayName: "nps内网穿透代理服务器",
			Description: "一款轻量级、功能强大的内网穿透代理服务器。支持tcp、udp流量转发，支持内网http代理、内网socks5代理，同时支持snappy压缩、站点保护、加密传输、多路复用、header修改等。支持web图形化管理，集成多用户模式。",
			Option:      options,
		}
		s, _ := service.New(prg, svcConfig)

		switch flag {
		case "1":
			// uninstall before
			_ = service.Control(s, "stop")
			_ = service.Control(s, "uninstall")
			binPath := install.InstallNpsToCurrentDir()

			// Ensure conf exists (same as normal startup), then load it for display.
			// Previously LoadAppConfig error was ignored and web_port could be empty,
			// printing "127.0.0.1:" with no port. See #317.
			confDir := filepath.Join(common.GetAppPath(), "conf")
			initConfig(confDir)
			if err := beego.LoadAppConfig("ini", filepath.Join(confDir, "nps.conf")); err != nil {
				fmt.Println("加载配置文件失败：", err)
				break
			}

			logPath := filepath.Join(common.GetAppPath(), "nps.log")
			if common.IsWindows() {
				logPath = strings.Replace(logPath, "\\", "\\\\", -1)
			}
			svcConfig.Arguments = append(svcConfig.Arguments, "service")
			svcConfig.Arguments = append(svcConfig.Arguments, "-conf_path="+common.GetAppPath())
			svcConfig.Arguments = append(svcConfig.Arguments, "-log_path="+logPath)

			fmt.Println("日志文件路径为：", logPath)

			svcConfig.Executable = binPath
			s, err := service.New(prg, svcConfig)

			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				confPath := "/etc/init.d/" + svcConfig.Name
				os.Symlink(confPath, "/etc/rc.d/S90"+svcConfig.Name)
				os.Symlink(confPath, "/etc/rc.d/K02"+svcConfig.Name)
			}

			err = service.Control(s, "install")
			if err != nil {
				logs.Error("Valid actions: %q\n%s", service.ControlAction, err.Error())
			} else {
				fmt.Println("NPS服务安装成功")
			}

			err = service.Control(s, "start")
			if err != nil {
				fmt.Println("启动NPS服务失败", err)
			} else {
				webPort := beego.AppConfig.String("web_port")
				if webPort == "" {
					fmt.Println("NPS服务已启动（未配置 web_port，管理面板已关闭）")
				} else {
					scheme := "http"
					if beego.AppConfig.DefaultBool("web_open_ssl", false) {
						scheme = "https"
					}
					fmt.Println("NPS服务已启动，管理面板访问地址：" + scheme + "://127.0.0.1:" + webPort)
				}
			}

			break
		case "2":
			// 卸载系统服务
			err := service.Control(s, "stop")
			if err != nil {
				fmt.Println("NPS服务停止失败", err)
			} else {
				fmt.Println("NPS服务已停止")
			}

			err = service.Control(s, "uninstall")
			if err != nil {
				logs.Error("NPS服务卸载失败")
			}
			if service.Platform() == "unix-systemv" {
				logs.Info("unix-systemv service")
				os.Remove("/etc/rc.d/S90" + svcConfig.Name)
				os.Remove("/etc/rc.d/K02" + svcConfig.Name)
			}

			if err == nil {
				fmt.Println("NPS服务已卸载成功")
			}
			break
		case "3":
			if updated, err := install.UpdateNps(); err != nil {
				fmt.Println("更新失败：", err)
			} else if updated {
				restartNpsService(s)
			}
			break
		case "4":
			// 启动 NPS
			if err := controlNpsService(s, "start"); err != nil {
				fmt.Println("NPS服务启动失败", err)
			} else {
				fmt.Println("NPS服务启动成功")
			}

			break
		case "5":
			// 停止 NPS
			if err := controlNpsService(s, "stop"); err != nil {
				fmt.Println("NPS服务停止失败", err)
			} else {
				fmt.Println("NPS服务停止成功")
			}

			break
		case "6":
			// 重启 NPS
			if err := controlNpsService(s, "restart"); err != nil {
				fmt.Println("NPS服务重启失败", err)
			} else {
				fmt.Println("NPS服务重启成功")
			}

			break
		}
	}

	inputCmd()
}

func installNps() {

}

type nps struct {
	exit chan struct{}
}

func (p *nps) Start(s service.Service) error {
	_, _ = s.Status()
	go p.run()
	return nil
}
func (p *nps) Stop(s service.Service) error {
	_, _ = s.Status()
	close(p.exit)
	if service.Interactive() {
		os.Exit(0)
	}
	return nil
}

func (p *nps) run() error {
	defer func() {
		if err := recover(); err != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logs.Warning("nps: panic serving %v: %v\n%s", err, string(buf))
		}
	}()
	run()
	select {
	case <-p.exit:
		logs.Warning("stop...")
	}
	return nil
}

// resolveGeoPaths 解析 conf 中的 geoip/geosite 路径（相对运行目录）。
func resolveGeoPaths() (ipPath, sitePath string) {
	ipPath = strings.TrimSpace(beego.AppConfig.String("geoip_path"))
	if ipPath == "" {
		ipPath = "conf/geoip.dat"
	}
	ipPath = common.ResolveConfigPath(ipPath)
	sitePath = strings.TrimSpace(beego.AppConfig.String("geosite_path"))
	if sitePath == "" {
		sitePath = filepath.Join(filepath.Dir(ipPath), "geosite.dat")
	} else {
		sitePath = common.ResolveConfigPath(sitePath)
	}
	return ipPath, sitePath
}

func initGeoData() {
	ipPath, sitePath := resolveGeoPaths()
	install.SetGeoDataPaths(ipPath, sitePath)
	acl.SetGeoData(ipPath, sitePath)
	if acl.GeoDataConfigured() {
		logs.Info("GeoIP 已加载: %s", ipPath)
	} else if _, err := os.Stat(ipPath); err != nil {
		logs.Info("GeoIP 未就绪（%s）；可用 nps -updategeo 或面板更新", ipPath)
	} else {
		logs.Warn("GeoIP 文件存在但解析失败: %s", ipPath)
	}
}

func run() {
	routers.Init()
	task := &file.Tunnel{
		Mode: "webServer",
	}
	bridgePort, err := beego.AppConfig.Int("bridge_port")
	if err != nil {
		logs.Error("Getting bridge_port error", err)
		os.Exit(0)
	}

	logs.Info("日志路径：" + *npsLogPath)
	logs.Info("the config path is:" + common.GetRunPath())
	logs.Info("the version of server is %s ,allow client core version to be %s,tls enable is %t", version.VERSION, version.GetVersion(), bridge.ServerTlsEnable)
	connection.InitConnectionService()
	bridgeCert := strings.TrimSpace(beego.AppConfig.String("bridge_cert_file"))
	bridgeKey := strings.TrimSpace(beego.AppConfig.String("bridge_key_file"))
	if bridgeCert != "" && bridgeKey != "" {
		if err := crypt.InitTlsFromFiles(common.ResolveConfigPath(bridgeCert), common.ResolveConfigPath(bridgeKey)); err != nil {
			logs.Warn("bridge certificate load failed, fallback to temporary self-signed: %v", err)
			crypt.InitTls()
		} else {
			logs.Info("bridge certificate loaded from configured files")
		}
	} else {
		crypt.InitTls()
	}
	tool.StartSystemInfo()
	timeout, err := beego.AppConfig.Int("disconnect_timeout")
	if err != nil {
		timeout = 60
	}
	go server.StartNewServer(bridgePort, task, beego.AppConfig.String("bridge_type"), timeout)
}

func initConfig(confDir string) {
	if !common.FileExists(confDir) {
		os.MkdirAll(confDir, 0755)
	}
	confPath := filepath.Join(confDir, "nps.conf")
	confExisted := common.FileExists(confPath)
	if !confExisted {
		webPassword := crypt.GetRandomString(8)
		authKey := crypt.GetRandomString(8)
		authCryptKey := crypt.GetRandomString(16)
		content := strings.Replace(defaultNpsConf, "web_password=123", "web_password="+webPassword, 1)
		content = strings.Replace(content, "auth_key=123", "auth_key="+authKey, 1)
		content = strings.Replace(content, "auth_crypt_key =213", "auth_crypt_key ="+authCryptKey, 1)
		f, err := os.Create(confPath)
		if err != nil {
			return
		}
		defer f.Close()
		f.WriteString(content)
		logs.Info("Auto-generated default config file:", confPath)
		logs.Info("Web login username: admin, password:", webPassword)
		logs.Info("auth_key:", authKey)
		logs.Info("auth_crypt_key:", authCryptKey)
	} else {
		// 存量 conf 只增迁移；不轮换已有密钥
		if changed, added, err := migrateConfigFile(confPath, defaultNpsConf); err != nil {
			logs.Error("migrate config file failed:", err)
		} else if changed && len(added) > 0 {
			logs.Info("Added missing settings to config file:", strings.Join(added, ", "))
		}
	}
	// 全新安装直接生成默认 global.json；升级场景交给 MigrateGlobalFromAppConfig
	// 从已有的 nps.conf 迁移，避免覆盖用户原有配置。
	if !confExisted {
		initGlobalConfig(confDir)
	}
}

// initGlobalConfig 生成默认的 conf/global.json。
// 可热更新的开关已从 nps.conf 迁移到这里，首次启动即生效。
func initGlobalConfig(confDir string) {
	globalPath := filepath.Join(confDir, "global.json")
	if common.FileExists(globalPath) {
		return
	}
	if err := os.WriteFile(globalPath, []byte(defaultGlobalJson), 0644); err != nil {
		logs.Error("Auto-generated default global config file error:", err)
		return
	}
	logs.Info("Auto-generated default global config file:", globalPath)
}

const defaultGlobalJson = `{"BlackIpList":[],"ServerUrl":"","AppName":"NPS","AllowPorts":"","IpLimit":false,"ShowHttpProxyPort":true,"AllowFlowLimit":true,"AllowRateLimit":true,"AllowConnectionNumLimit":true,"AllowTunnelNumLimit":true,"AllowMultiIp":true,"AllowLocalProxy":false,"AllowUserLogin":true,"AllowUserRegister":false,"AllowUserChangeUsername":true,"OpenCaptcha":false,"SystemInfoDisplay":true,"HttpAddOriginHeader":true}`

const defaultNpsConf = `http_proxy_ip=0.0.0.0
http_proxy_port=80
https_proxy_port=443

bridge_type=tcp
bridge_port=8024
bridge_ip=0.0.0.0

public_vkey=123

flow_store_interval=1

log_level=6
log_path=nps.log

web_host=a.o.com
web_username=admin
web_password=123
web_port = 8081
web_ip=0.0.0.0
web_base_url=
web_open_ssl=false
web_cert_file=conf/server.pem
web_key_file=conf/server.key
# --- 面板展示：顶栏环境标识 / 侧栏副标题；改后需重启 ---
# web_welcome=DNS二线
# web_subtitle=内网穿透控制台
# --- 管理员登录 TOTP（空=关闭；nps -gen2fa 生成后填入并重启）---
# totp_secret=
# --- 安全入口（空=关闭；非空须先访问该路径解锁 Session 后才显示登录页；改后需重启）---
# web_secure_entry=/s/请换成长随机串?
auth_key=123
auth_crypt_key =213

http_cache=false
http_cache_length=100

disconnect_timeout=60

tls_enable=true
tls_bridge_port=8025

# --- 以下为融合可选旁路（未配置端口则不监听，不影响旧 npc）---
# bridge_kcp_port=
# bridge_ws_port=
# bridge_wss_port=
# bridge_quic_port=   # UDP QUIC 旁路；npc -type=quic -server=host:port
# bridge_path=/ws
# bridge_select_mode=primary
# bridge_require_tls=false
# bridge_secure_handshake=false
# bridge_cert_file=
# bridge_key_file=
# --- ACME 自动证书（默认不申请；Host 需开启 AutoSSL）---
# ssl_email=
# ssl_path=conf/ssl
# force_auto_ssl=false
# auth_time_window=300
# 管理面审计：空/未配置=关闭；写失败不影响转发
# audit_log_path=
# 连接审计（上下线）；需同时配置 audit_log_path；默认关
# audit_connection=false
# 同一 client+动作最短写间隔（秒）；0=不去抖；默认 30
# audit_connection_min_interval=30
# max_clients=0
# max_tunnels_per_client=0
# max_global_conn=0
# --- HTTP/3（UDP；需同时启用 https_proxy_port；空=不监听）---
# http3_proxy_port=
# https_default_cert_file=conf/server.pem
# https_default_key_file=conf/server.key
# --- 真实 IP（域名 HTTP；内置多 CDN 头）---
# allow_x_real_ip=true
# trusted_proxy_ips=
# --- 远程升级 npc（默认关；开启后面板可下发）---
# allow_remote_update=false
# --- P2P（服务端提示；stun 辅助由 npc -p2p_mode=stun）---
# p2p_mode=legacy
# p2p_stun_servers=stun.stunprotocol.org:3478
# --- 注册增强（AllowUserRegister 。global.json；默认关。--
# register_rate_limit=5
# register_email_verify=false
# smtp_host=
# smtp_port=587
# smtp_security=starttls
# smtp_user=
# smtp_pass=
# smtp_from=
# --- 面板终端（默认关）---
# allow_web_terminal=false
# --- 自建下载源（空=GitHub cloudwebsite/nps）---
# release_api_url=
# release_download_base=
# --- WAF / GeoIP（默认全关；waf_protect_panel 默认开）---
# waf_enable=false
# waf_protect_panel=true
# waf_ua_blacklist_enable=false
# waf_block_foreign_enable=false
# waf_custom_rules_enable=false
# waf_block_foreign_unknown=true
# waf_country_headers=CF-IPCountry
# geoip_path=conf/geoip.dat
# geosite_path=conf/geosite.dat
# waf_rules_path=conf/waf.json
`
