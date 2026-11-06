package notify

import (
	"sync"
	"time"
)

// RateLimiter 按 key 最短间隔去抖。
type RateLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func newRateLimiter() *RateLimiter {
	return &RateLimiter{last: make(map[string]time.Time)}
}

// Allow 同一 key 距上次成功放行不足 min 则拒绝；min<=0 表示不去抖。
func (r *RateLimiter) Allow(key string, min time.Duration) bool {
	if r == nil {
		return true
	}
	if min <= 0 {
		return true
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.last[key]; ok && now.Sub(prev) < min {
		return false
	}
	r.last[key] = now
	return true
}
