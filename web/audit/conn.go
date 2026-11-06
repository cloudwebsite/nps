package audit

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// connectionEnabled 连接审计开关；默认关，避免管理面审计路径被上下线刷爆。
var connectionEnabled atomic.Bool

// connectionMinInterval 同一 client+action 的最短写间隔；0=不去抖。
var connectionMinInterval atomic.Int64

var lastConnectionWrite sync.Map // key -> unix nano

// SetConnectionEnabled 开启/关闭 Bridge 连接审计写点。
func SetConnectionEnabled(on bool) {
	connectionEnabled.Store(on)
}

// ConnectionEnabled 当前是否记录连接审计。
func ConnectionEnabled() bool {
	return connectionEnabled.Load()
}

// SetConnectionMinInterval 设置连接审计去抖间隔（秒；<0 按 0）。
func SetConnectionMinInterval(seconds int) {
	if seconds < 0 {
		seconds = 0
	}
	connectionMinInterval.Store(int64(time.Duration(seconds) * time.Second))
}

// ConnectionMinInterval 当前去抖间隔。
func ConnectionMinInterval() time.Duration {
	return time.Duration(connectionMinInterval.Load())
}

// RecordConnection 记录客户端上下线（失败不得影响转发；可去抖）。
func RecordConnection(action string, clientID int, ownerUserID int, addr string, details map[string]string) {
	if !connectionEnabled.Load() || Path() == "" {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return
	}
	min := ConnectionMinInterval()
	if min > 0 {
		key := fmt.Sprintf("%d:%s", clientID, action)
		now := time.Now().UnixNano()
		if prev, ok := lastConnectionWrite.Load(key); ok {
			if now-prev.(int64) < int64(min) {
				return
			}
		}
		lastConnectionWrite.Store(key, now)
	}
	if details == nil {
		details = map[string]string{}
	}
	if addr != "" {
		details["addr"] = addr
	}
	details["client_id"] = strconv.Itoa(clientID)
	_ = Append(Event{
		ActorType:    "bridge",
		ActorName:    "npc",
		Action:       action,
		ResourceType: "client",
		ResourceID:   clientID,
		OwnerUserID:  ownerUserID,
		Result:       ResultSuccess,
		Details:      details,
	})
}
