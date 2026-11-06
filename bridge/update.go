package bridge

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/version"
)

const remoteUpdateWaitTimeout = 120 * time.Second

// ErrUpdateUnsupported 客户端产品版本低于远程升级地板，禁止下发 CLIENT_UPDATE。
var ErrUpdateUnsupported = fmt.Errorf(
	"客户端版本不支持远程升级，需要 npc %s 或更高（空版本/旧版本不会收到升级信令）",
	version.RemoteUpdateMinVersion,
)

// RemoteUpdateOutcome 远程升级等待客户端回传后的结果。
type RemoteUpdateOutcome struct {
	Status  string // ok | skip | fail | timeout
	Channel string // cli | gui
	Message string
}

// clientSupportsRemoteUpdate 按 Bridge 会话上报的产品版本门控。
func clientSupportsRemoteUpdate(productVersion string) bool {
	return version.SupportsRemoteUpdate(productVersion)
}

// RequestClientUpdate 向在线且版本达标的 npc 下发 CLIENT_UPDATE + release 源，并等待回传。
// 须 allow_remote_update=true；旧/空版本绝不发送，避免未知信令导致重连。
func (s *Bridge) RequestClientUpdate(id int) (RemoteUpdateOutcome, error) {
	var out RemoteUpdateOutcome
	if !file.EffectiveAllowRemoteUpdate() {
		return out, errors.New("服务端已全局禁止远程升级（allow_remote_update=false）")
	}
	if s == nil {
		return out, errors.New("服务端 Bridge 尚未初始化")
	}
	v, ok := s.Client.Load(id)
	if !ok {
		return out, fmt.Errorf("客户端 %d 当前未连接", id)
	}
	cl := v.(*Client)
	cl.mu.Lock()
	signal := cl.signal
	vs := cl.Version
	cl.mu.Unlock()
	if signal == nil || signal.Conn == nil {
		return out, fmt.Errorf("客户端 %d 当前未连接", id)
	}
	if !clientSupportsRemoteUpdate(vs) {
		return out, ErrUpdateUnsupported
	}

	waiter := make(chan RemoteUpdateOutcome, 1)
	cl.mu.Lock()
	cl.updateWaiter = waiter
	cl.mu.Unlock()
	defer func() {
		cl.mu.Lock()
		if cl.updateWaiter == waiter {
			cl.updateWaiter = nil
		}
		cl.mu.Unlock()
	}()

	apiURL, downloadBase := install.ReleaseSourceSnapshot()
	if _, err := signal.Write([]byte(common.CLIENT_UPDATE)); err != nil {
		return out, fmt.Errorf("发送升级指令失败：%w", err)
	}
	if err := signal.WriteLenContent([]byte(apiURL)); err != nil {
		return out, fmt.Errorf("发送升级源 API 失败：%w", err)
	}
	if err := signal.WriteLenContent([]byte(downloadBase)); err != nil {
		return out, fmt.Errorf("发送升级源下载前缀失败：%w", err)
	}

	select {
	case out = <-waiter:
		storeClientUpdateResult(id, out)
		return out, nil
	case <-time.After(remoteUpdateWaitTimeout):
		out = RemoteUpdateOutcome{
			Status:  "timeout",
			Message: "指令已下发并附带当前 Release 源，但未在时限内收到客户端回传（下载较慢或进程已退出）；请查看客户端日志或稍后刷新列表",
		}
		storeClientUpdateResult(id, out)
		return out, nil
	}
}

func (s *Bridge) deliverRemoteUpdateOutcome(id int, out RemoteUpdateOutcome) {
	storeClientUpdateResult(id, out)
	v, ok := s.Client.Load(id)
	if !ok {
		return
	}
	cl := v.(*Client)
	cl.mu.Lock()
	ch := cl.updateWaiter
	cl.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- out:
	default:
	}
}

func storeClientUpdateResult(id int, out RemoteUpdateOutcome) {
	c, err := file.GetDb().GetClient(id)
	if err != nil || c == nil {
		return
	}
	c.Lock()
	c.LastUpdateStatus = out.Status
	c.LastUpdateChannel = out.Channel
	c.LastUpdateMsg = out.Message
	c.LastUpdateAt = time.Now().Format("2006-01-02 15:04:05")
	c.Unlock()
}

// ParseRemoteUpdateReport 解析健康通道中的升级回传 info。
func ParseRemoteUpdateReport(info string) (RemoteUpdateOutcome, bool) {
	info = strings.TrimSpace(info)
	if !strings.HasPrefix(info, common.RemoteUpdateReportPrefix) {
		return RemoteUpdateOutcome{}, false
	}
	rest := strings.TrimPrefix(info, common.RemoteUpdateReportPrefix)
	rest = strings.TrimPrefix(rest, "|")
	parts := strings.SplitN(rest, "|", 3)
	out := RemoteUpdateOutcome{Status: "fail"}
	if len(parts) >= 1 {
		out.Status = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		out.Channel = strings.TrimSpace(parts[1])
	}
	if len(parts) >= 3 {
		out.Message = strings.TrimSpace(parts[2])
	}
	if out.Status == "" {
		out.Status = "fail"
	}
	return out, true
}
