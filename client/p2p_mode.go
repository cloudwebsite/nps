package client

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/astaxie/beego/logs"
	"github.com/ccding/go-stun/stun"
)

// P2PMode 控制打洞与直连传输。
// legacy：现网 UDP 打洞 + KCP（默认）
// stun：打洞前 STUN 探测，传输仍为 KCP
// modern：STUN + 旧 UDP 打洞成功后改用 QUIC mux；可选 -p2p_holepunch 走 djylb 协调器；失败则回退经典打洞或 Secret
	// 不向旧协议发新控制帧；两端须同为 modern 才走 QUIC。
type P2PMode int32

const (
	P2PModeLegacy P2PMode = iota
	P2PModeSTUN
	P2PModeModern
)

var p2pMode atomic.Int32
var p2pStunServer atomic.Value // string

func init() {
	p2pStunServer.Store("stun.stunprotocol.org:3478")
}

// SetP2PMode 解析 p2p_mode / -p2p_mode。
func SetP2PMode(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "legacy", "old", "0":
		p2pMode.Store(int32(P2PModeLegacy))
		return nil
	case "stun", "stun_assist", "1":
		p2pMode.Store(int32(P2PModeSTUN))
		return nil
	case "modern", "quic", "holepunch", "2":
		p2pMode.Store(int32(P2PModeModern))
		return nil
	default:
		p2pMode.Store(int32(P2PModeLegacy))
		return fmt.Errorf("无效的 p2p_mode=%q（允许 legacy/stun/modern）", value)
	}
}

func GetP2PMode() P2PMode {
	switch P2PMode(p2pMode.Load()) {
	case P2PModeSTUN:
		return P2PModeSTUN
	case P2PModeModern:
		return P2PModeModern
	default:
		return P2PModeLegacy
	}
}

func P2PModeName() string {
	switch GetP2PMode() {
	case P2PModeSTUN:
		return "stun"
	case P2PModeModern:
		return "modern"
	default:
		return "legacy"
	}
}

// SetP2PStunServer 设置 STUN 服务器（空则保留默认）。
func SetP2PStunServer(addr string) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return
	}
	p2pStunServer.Store(addr)
}

func GetP2PStunServer() string {
	if v, ok := p2pStunServer.Load().(string); ok && v != "" {
		return v
	}
	return "stun.stunprotocol.org:3478"
}

// discoverP2PLocalAddr STUN 辅助：探测 NAT；stun/modern 模式下启用。
// 失败时返回 fallback，不中断旧路径。
func discoverP2PLocalAddr(fallback string) string {
	mode := GetP2PMode()
	if mode != P2PModeSTUN && mode != P2PModeModern {
		return fallback
	}
	c := stun.NewClient()
	c.SetServerAddr(GetP2PStunServer())
	nat, host, err := c.Discover()
	if err != nil || host == nil {
		logs.Warn("P2P STUN 探测失败，继续用本地地址：%v", err)
		return fallback
	}
	logs.Info("P2P STUN：nat=%s public=%s mode=%s", nat.String(), host.String(), P2PModeName())
	if strings.TrimSpace(fallback) != "" {
		if _, _, err := net.SplitHostPort(fallback); err == nil {
			return fallback
		}
	}
	return host.String()
}
