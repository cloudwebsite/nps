package tool

import (
	"sync"
	"time"

	"ehang.io/nps/lib/idle"
)

// 隧道闲置自动停用：taskID → IdleTimer（仅 tcp/udp 由 server 侧 arm）。
var idleAutoStops sync.Map

// ArmIdleAutoStop 为隧道挂闲置计时；minutes≤0 或 onIdle 为空则仅卸旧计时器。
func ArmIdleAutoStop(taskID, minutes int, onIdle func()) {
	if minutes <= 0 {
		DisarmIdleAutoStop(taskID)
		return
	}
	ArmIdleAutoStopDuration(taskID, time.Duration(minutes)*time.Minute, onIdle)
}

// ArmIdleAutoStopDuration 供测试与内部使用。
func ArmIdleAutoStopDuration(taskID int, idleDur time.Duration, onIdle func()) {
	DisarmIdleAutoStop(taskID)
	if taskID <= 0 || idleDur <= 0 || onIdle == nil {
		return
	}
	it := idle.New(idleDur, onIdle)
	idleAutoStops.Store(taskID, it)
}

// DisarmIdleAutoStop 取消并移除计时器。
func DisarmIdleAutoStop(taskID int) {
	if v, ok := idleAutoStops.LoadAndDelete(taskID); ok {
		if it, ok := v.(*idle.Timer); ok {
			it.Stop()
		}
	}
}

// IdleAutoStopInc 标记有活动业务连接。
func IdleAutoStopInc(taskID int) {
	if v, ok := idleAutoStops.Load(taskID); ok {
		if it, ok := v.(*idle.Timer); ok {
			it.Inc()
		}
	}
}

// IdleAutoStopDec 活动连接结束。
func IdleAutoStopDec(taskID int) {
	if v, ok := idleAutoStops.Load(taskID); ok {
		if it, ok := v.(*idle.Timer); ok {
			it.Dec()
		}
	}
}
