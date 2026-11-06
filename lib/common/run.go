package common

import (
	"os"
	"path/filepath"
	"runtime"
)

var ConfPath string

// Get the currently selected configuration file directory
// For non-Windows systems, select the /etc/nps as config directory if exist, or select ./
// windows system, select the C:\Program Files\nps as config directory if exist, or select ./
func GetRunPath() string {
	if ConfPath != "" {
		return ConfPath
	}

	var path string
	if len(os.Args) == 1 {
		if !IsWindows() {
			dir, _ := filepath.Abs(filepath.Dir(os.Args[0])) //返回
			return dir + "/"
		} else {
			return "./"
		}
	} else {
		if path = GetInstallPath(); !FileExists(path) {
			return GetAppPath()
		}
	}
	return path
}

// Different systems get different installation paths
func GetInstallPath() string {
	var path string

	if ConfPath != "" {
		return ConfPath
	}

	if IsWindows() {
		path = `C:\Program Files\nps`
	} else {
		path = "/etc/nps"
	}

	return path
}

// GetExecutablePath 返回当前进程可执行文件的绝对路径（解析符号链接）。
// 经 PATH 调用时 os.Args[0] 可能只是 "nps"，不能用来定位安装目录。
func GetExecutablePath() string {
	exe, err := os.Executable()
	if err == nil {
		if resolved, err2 := filepath.EvalSymlinks(exe); err2 == nil {
			exe = resolved
		}
		if abs, err2 := filepath.Abs(exe); err2 == nil {
			return abs
		}
		return exe
	}
	if abs, err2 := filepath.Abs(os.Args[0]); err2 == nil {
		return abs
	}
	return os.Args[0]
}

// GetAppPath 返回当前可执行文件所在目录（不是 shell 的 cwd）。
func GetAppPath() string {
	return filepath.Dir(GetExecutablePath())
}

// Determine whether the current system is a Windows system?
func IsWindows() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}

// interface log file path
func GetLogPath() string {
	var path string
	if IsWindows() {
		path = filepath.Join(GetAppPath(), "nps.log")
	} else {
		path = "/var/log/nps.log"
	}
	return path
}

func GetLogPathCurrentPath() string {
	var path string
	path = filepath.Join(GetAppPath(), "nps.log")
	return path
}

// interface npc log file path
func GetNpcLogPath() string {
	var path string
	if IsWindows() {
		path = filepath.Join(GetAppPath(), "npc.log")
	} else {
		path = "/var/log/npc.log"
	}
	return path
}

// interface pid file path
func GetTmpPath() string {
	var path string
	if IsWindows() {
		path = GetAppPath()
	} else {
		path = "/tmp"
	}
	return path
}

// config file path
func GetConfigPath() string {
	var path string
	if IsWindows() {
		path = filepath.Join(GetAppPath(), "conf/npc.conf")
	} else {
		path = "conf/npc.conf"
	}
	return path
}
