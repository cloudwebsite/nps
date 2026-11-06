package idle

import (
	"sync"
	"sync/atomic"
	"time"
)

// Timer 在无活动连接时于 idle 时长后调用 closeFn（改编自 djylb）。
type Timer struct {
	idle    time.Duration
	closeFn func()
	mu      sync.Mutex
	active  int
	t       *time.Timer
	closed  atomic.Uint32
}

// New 创建空闲计时器；idle<=0 时不会自动触发 closeFn。
func New(idle time.Duration, closeFn func()) *Timer {
	it := &Timer{idle: idle, closeFn: closeFn}
	if idle <= 0 || closeFn == nil {
		it.closed.Store(1)
		return it
	}
	it.t = time.AfterFunc(idle, func() {
		shouldClose := false
		it.mu.Lock()
		if it.active == 0 && it.closed.CompareAndSwap(0, 1) {
			shouldClose = true
		}
		it.mu.Unlock()
		if shouldClose {
			closeFn()
		}
	})
	return it
}

// Inc 标记有活动连接；首次进入活动态时停止计时。
func (it *Timer) Inc() {
	if it == nil || it.closed.Load() == 1 {
		return
	}
	it.mu.Lock()
	it.active++
	if it.active == 1 && it.t != nil {
		_ = it.t.Stop()
	}
	it.mu.Unlock()
}

// Dec 活动连接结束；归零后重新开始空闲倒计时。
func (it *Timer) Dec() {
	if it == nil || it.closed.Load() == 1 {
		return
	}
	it.mu.Lock()
	if it.active > 0 {
		it.active--
		if it.active == 0 && it.closed.Load() == 0 && it.t != nil {
			it.t.Reset(it.idle)
		}
	}
	it.mu.Unlock()
}

// Stop 永久停止计时器，不再调用 closeFn。
func (it *Timer) Stop() {
	if it == nil {
		return
	}
	if it.closed.CompareAndSwap(0, 1) && it.t != nil {
		_ = it.t.Stop()
	}
}
