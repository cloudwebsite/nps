package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/p2pholepunch"
	"github.com/astaxie/beego/logs"
)

var p2pHolepunchAddr atomic.Value // string host:port

// SetP2PHolepunchAddr 设置 holepunch 协调器地址（空=不启用）。
func SetP2PHolepunchAddr(addr string) {
	addr = strings.TrimSpace(addr)
	p2pHolepunchAddr.Store(addr)
}

func GetP2PHolepunchAddr() string {
	if v, ok := p2pHolepunchAddr.Load().(string); ok {
		return v
	}
	return ""
}

// tryHolepunchQUIC 经协调器打洞后建立 QUIC mux 底层连接。
func tryHolepunchQUIC(password, role string) (net.Conn, error) {
	addr := GetP2PHolepunchAddr()
	if addr == "" || GetP2PMode() != P2PModeModern {
		return nil, fmt.Errorf("holepunch not enabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	sess, err := p2pholepunch.DialAndPunch(ctx, addr, password, role)
	if err != nil {
		return nil, err
	}
	pc := sess.PacketConn()
	if pc == nil {
		_ = sess.Close()
		return nil, fmt.Errorf("holepunch: empty packet conn")
	}
	remote := ""
	if res := sess.Result(); res.Path.RemoteAddr.IsValid() {
		remote = res.Path.RemoteAddr.String()
	}
	sess.Detach()
	if role == "visitor" {
		qc, err := dialP2PQUIC(pc, remote)
		if err != nil {
			_ = pc.Close()
			return nil, err
		}
		logs.Info("P2P modern：holepunch+QUIC 访客就绪 remote=%s", remote)
		return qc, nil
	}
	qc, err := acceptP2PQUIC(pc, remote)
	if err != nil {
		_ = pc.Close()
		return nil, err
	}
	logs.Info("P2P modern：holepunch+QUIC 提供方就绪 remote=%s", remote)
	return qc, nil
}
