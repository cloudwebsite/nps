package tool

import (
	"errors"
	"net"
	"sync"
)

// TunnelDialer 运行中的 TCP 类隧道可接受进程内虚拟连接（tunnel://）。
type TunnelDialer interface {
	DialVirtual(remote string) (net.Conn, error)
}

var tunnelLookup struct {
	sync.RWMutex
	fn func(int) (TunnelDialer, bool)
}

// SetTunnelLookup 安装任务查找回调，避免 bridge 与 proxy 循环依赖。
func SetTunnelLookup(fn func(int) (TunnelDialer, bool)) {
	tunnelLookup.Lock()
	tunnelLookup.fn = fn
	tunnelLookup.Unlock()
}

// GetTunnelConn 按隧道 ID 注入虚拟连接；未启用或目标不可用时返回错误。
func GetTunnelConn(id int, remote string) (net.Conn, error) {
	tunnelLookup.RLock()
	lookup := tunnelLookup.fn
	tunnelLookup.RUnlock()
	if lookup == nil {
		return nil, errors.New("tunnel lookup is not configured")
	}
	dialer, ok := lookup(id)
	if !ok || dialer == nil {
		return nil, errors.New("target tunnel is unavailable or not eligible for nested forwarding")
	}
	return dialer.DialVirtual(remote)
}
