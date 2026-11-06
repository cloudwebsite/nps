package idle

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerFiresWhenIdle(t *testing.T) {
	var fired atomic.Uint32
	it := New(40*time.Millisecond, func() { fired.Store(1) })
	defer it.Stop()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fired.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected idle closeFn")
}

func TestTimerIncPreventsFire(t *testing.T) {
	var fired atomic.Uint32
	it := New(50*time.Millisecond, func() { fired.Store(1) })
	defer it.Stop()
	it.Inc()
	time.Sleep(120 * time.Millisecond)
	if fired.Load() == 1 {
		t.Fatal("active connection must not idle-close")
	}
	it.Dec()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fired.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected fire after Dec back to idle")
}

func TestTimerStop(t *testing.T) {
	var fired atomic.Uint32
	it := New(30*time.Millisecond, func() { fired.Store(1) })
	it.Stop()
	time.Sleep(80 * time.Millisecond)
	if fired.Load() == 1 {
		t.Fatal("Stop must prevent closeFn")
	}
}

func TestTimerZeroDisabled(t *testing.T) {
	var fired atomic.Uint32
	it := New(0, func() { fired.Store(1) })
	it.Inc()
	it.Dec()
	time.Sleep(50 * time.Millisecond)
	if fired.Load() == 1 {
		t.Fatal("idle<=0 must not fire")
	}
}
