//go:build windows

package client

import (
	"io"
	"net"
	"os"
	"os/exec"
	"sync"

	"ehang.io/nps/lib/conn"
	"github.com/UserExistsError/conpty"
)

var shellConPtyMap sync.Map

// Windows：优先 ConPTY；失败回退 cmd 管道。
func handleShellLink(s *TRPClient, src net.Conn, lk *conn.Link) {
	cols, rows := lk.Cols, lk.Rows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	home := shellHome()
	hostname, _ := os.Hostname()
	env := buildShellEnv(os.Environ(), home, hostname)

	cpty, err := conpty.Start(`c:\windows\system32\cmd.exe`,
		conpty.ConPtyDimensions(cols, rows),
		conpty.ConPtyWorkDir(home),
		conpty.ConPtyEnv(env),
	)
	if err != nil {
		s.logWarn("ConPTY 不可用，回退管道: %v", err)
		handleShellLinkPipes(s, src, lk, home, env)
		return
	}
	if lk.ShellID != "" {
		shellConPtyMap.Store(lk.ShellID, cpty)
		defer shellConPtyMap.Delete(lk.ShellID)
	}
	srcConn := conn.GetConn(src, lk.Crypt, lk.Compress, nil, false)
	go func() {
		io.Copy(srcConn, cpty)
		srcConn.Close()
		_ = cpty.Close()
	}()
	io.Copy(cpty, srcConn)
	_ = cpty.Close()
	srcConn.Close()
}

func handleShellLinkPipes(s *TRPClient, src net.Conn, lk *conn.Link, home string, env []string) {
	cmd := exec.Command("cmd.exe")
	cmd.Dir = home
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		s.logWarn("start shell stdin error %s", err.Error())
		src.Close()
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		s.logWarn("start shell stdout error %s", err.Error())
		src.Close()
		return
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		s.logWarn("start shell error %s", err.Error())
		src.Close()
		return
	}
	srcConn := conn.GetConn(src, lk.Crypt, lk.Compress, nil, false)
	go func() {
		io.Copy(srcConn, stdout)
		srcConn.Close()
		_ = cmd.Process.Kill()
	}()
	io.Copy(stdin, srcConn)
	_ = stdin.Close()
	srcConn.Close()
	_ = cmd.Process.Kill()
}

func handleShellResize(lk *conn.Link) {
	if lk == nil || lk.ShellID == "" {
		return
	}
	if v, ok := shellConPtyMap.Load(lk.ShellID); ok {
		if cpty, ok2 := v.(*conpty.ConPty); ok2 {
			cols, rows := lk.Cols, lk.Rows
			if cols <= 0 {
				cols = 80
			}
			if rows <= 0 {
				rows = 24
			}
			_ = cpty.Resize(cols, rows)
		}
	}
}
