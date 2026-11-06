package bridge

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

// ParseRequireTLS 解析 bridge_require_tls。
// 空 / false / off / auto → false（升级默认保留明文口）；true / on → true。
func ParseRequireTLS(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	default:
		// 含 "" / false / off / auto：无损升级优先
		return false
	}
}

// SelectMode 控制同 VerifyKey 多 WORK_CHAN 时的选取策略。
// Primary：后来者替换（与旧单连接一致）；RoundRobin / Random：保留多隧道并分流。
type SelectMode int32

const (
	Primary SelectMode = iota
	RoundRobin
	Random
)

var clientSelectMode atomic.Int32

func SetClientSelectMode(value string) error {
	mode := Primary
	switch text := strings.ToLower(strings.TrimSpace(value)); text {
	case "", "primary", "p", "0":
		mode = Primary
	case "roundrobin", "round-robin", "round", "rr", "1":
		mode = RoundRobin
	case "random", "rand", "2":
		mode = Random
	default:
		if number, err := strconv.Atoi(text); err == nil && number >= int(Primary) && number <= int(Random) {
			mode = SelectMode(number)
		} else {
			clientSelectMode.Store(int32(Primary))
			return fmt.Errorf("无效的 bridge_select_mode=%q", value)
		}
	}
	clientSelectMode.Store(int32(mode))
	return nil
}

func ClientSelectMode() SelectMode {
	mode := SelectMode(clientSelectMode.Load())
	if mode < Primary || mode > Random {
		return Primary
	}
	return mode
}

func ClientSelectModeName() string {
	switch ClientSelectMode() {
	case RoundRobin:
		return "roundrobin"
	case Random:
		return "random"
	default:
		return "primary"
	}
}
