package tool

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestArmIdleAutoStopFires(t *testing.T) {
	var fired atomic.Int32
	ArmIdleAutoStopDuration(9101, 40*time.Millisecond, func() {
		fired.Store(1)
	})
	defer DisarmIdleAutoStop(9101)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fired.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("idle auto-stop did not fire")
}

func TestIdleAutoStopIncDecPreventsFire(t *testing.T) {
	var fired atomic.Int32
	ArmIdleAutoStopDuration(9102, 80*time.Millisecond, func() {
		fired.Store(1)
	})
	defer DisarmIdleAutoStop(9102)
	IdleAutoStopInc(9102)
	time.Sleep(120 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatal("active connection should prevent idle fire")
	}
	IdleAutoStopDec(9102)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if fired.Load() == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected fire after Dec to zero")
}

func TestArmIdleAutoStopInvalid(t *testing.T) {
	var fired atomic.Int32
	ArmIdleAutoStop(0, 1, func() { fired.Store(1) })
	ArmIdleAutoStop(9103, 0, func() { fired.Store(2) })
	time.Sleep(30 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatal("invalid arm should not fire")
	}
}
