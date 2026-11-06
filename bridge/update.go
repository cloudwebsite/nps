package bridge

import (
	"errors"
	"fmt"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
)

// ErrUpdateUnsupported 客户端产品版本低于远程升级地板，禁止下发 CLIENT_UPDATE。
var ErrUpdateUnsupported = fmt.Errorf(
	"客户端版本不支持远程升级，需要 npc %s 或更高（空版本/旧版本不会收到升级信令）",
	version.RemoteUpdateMinVersion,
)

// clientSupportsRemoteUpdate 按 Bridge 会话上报的产品版本门控。
func clientSupportsRemoteUpdate(productVersion string) bool {
	return version.SupportsRemoteUpdate(productVersion)
}

// RequestClientUpdate 向在线且版本达标的 npc 下发 CLIENT_UPDATE。
// 须 allow_remote_update=true；旧/空版本绝不发送，避免未知信令导致重连。
func (s *Bridge) RequestClientUpdate(id int) error {
	if !file.EffectiveAllowRemoteUpdate() {
		return errors.New("服务端已全局禁止远程升级（allow_remote_update=false）")
	}
	if s == nil {
		return errors.New("服务端 Bridge 尚未初始化")
	}
	v, ok := s.Client.Load(id)
	if !ok {
		return fmt.Errorf("客户端 %d 当前未连接", id)
	}
	cl := v.(*Client)
	cl.mu.Lock()
	signal := cl.signal
	vs := cl.Version
	cl.mu.Unlock()
	if signal == nil || signal.Conn == nil {
		return fmt.Errorf("客户端 %d 当前未连接", id)
	}
	if !clientSupportsRemoteUpdate(vs) {
		return ErrUpdateUnsupported
	}
	if _, err := signal.Write([]byte(common.CLIENT_UPDATE)); err != nil {
		return fmt.Errorf("发送升级指令失败：%w", err)
	}
	return nil
}
