package bridge

import (
	"time"

	"ehang.io/nps/lib/idle"
)

// IdleTimer 兼容别名：实现已下沉到 lib/idle，避免 bridge ↔ server/tool 循环依赖。
type IdleTimer = idle.Timer

// NewIdleTimer 创建空闲计时器。
func NewIdleTimer(d time.Duration, closeFn func()) *IdleTimer {
	return idle.New(d, closeFn)
}
