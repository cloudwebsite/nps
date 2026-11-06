//go:build !windows

package client

import (
	"io"
	"net"
	"os"
	"os/exec"
	"sync"

	"ehang.io/nps/lib/conn"
	"github.com/creack/pty"
)

var shellPtyMap sync.Map

func handleShellLink(s *TRPClient, src net.Conn, lk *conn.Link) {
	cols, rows := lk.Cols, lk.Rows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	shellPath := "/bin/sh"
	if _, err := os.Stat("/bin/bash"); err == nil {
		shellPath = "/bin/bash"
	}
	cmd := exec.Command(shellPath, "-l")
	home := shellHome()
	cmd.Dir = home
	hostname, _ := os.Hostname()
	cmd.Env = buildShellEnv(os.Environ(), home, hostname)
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		s.logWarn("start shell error %s", err.Error())
		src.Close()
		return
	}
	if lk.ShellID != "" {
		shellPtyMap.Store(lk.ShellID, f)
		defer shellPtyMap.Delete(lk.ShellID)
	}
	srcConn := conn.GetConn(src, lk.Crypt, lk.Compress, nil, false)
	go func() {
		io.Copy(srcConn, f)
		srcConn.Close()
		f.Close()
		_ = cmd.Process.Kill()
	}()
	io.Copy(f, srcConn)
	f.Close()
	srcConn.Close()
	_ = cmd.Process.Kill()
}

func handleShellResize(lk *conn.Link) {
	if lk == nil || lk.ShellID == "" {
		return
	}
	if pf, ok := shellPtyMap.Load(lk.ShellID); ok {
		if f, ok2 := pf.(*os.File); ok2 {
			_ = pty.Setsize(f, &pty.Winsize{Cols: uint16(lk.Cols), Rows: uint16(lk.Rows)})
		}
	}
}
