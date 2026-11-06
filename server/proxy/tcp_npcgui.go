//go:build npcgui
// +build npcgui

package proxy

import (
	"errors"

	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
)

// GUI 客户端构建（-tags npcgui）不包含 tcp.go 中的完整隧道服务，
// 此处提供占位实现，保证 client/local.go 等引用可以编译。

type process func(c *conn.Conn, s *TunnelModeServer) error

type TunnelModeServer struct{}

func NewTunnelModeServer(p process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	return new(TunnelModeServer)
}

func (s *TunnelModeServer) Start() error {
	return errors.New("tunnel mode server is not available in GUI build")
}

func (s *TunnelModeServer) Close() error {
	return nil
}

// ProcessMix 占位，避免 GUI 构建引用缺失符号。
func ProcessMix(c *conn.Conn, s *TunnelModeServer) error {
	if c != nil {
		_ = c.Close()
	}
	return errors.New("mixed proxy server is not available in GUI build")
}
