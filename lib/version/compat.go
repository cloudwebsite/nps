package version

import (
	"strconv"
	"strings"
	"sync/atomic"
)

// 握手版本分流。
// 编译门槛 SecureHandshakeMinVersion 表示「本族 npc 从哪一版起会说挑战握手」。
// 服务端另有 bridge_secure_handshake 开关（默认关）：关时即使版本达标也只走 legacy，
// 保证「先升 nps、旧 npc 仍连」；开后仅达标客户端走挑战握手。

// SecureHandshakeMinVersion 启用挑战握手的最低客户端产品版本（点号比较；多于三段时按段比较）。
const SecureHandshakeMinVersion = "0.26.41"

	// RemoteUpdateMinVersion 是能处理 CLIENT_UPDATE（含 release 源载荷 + 回传）的最低产品版本。
	// 更旧或空版本号的 npc 会把未知信令当断流并重连，故 Bridge 禁止下发。
	// 0.26.41.3 起：updt 后附带 apiURL/downloadBase，并经健康通道回传结果。
	const RemoteUpdateMinVersion = "0.26.41.3"

var secureHandshakeServerEnabled atomic.Bool

// SetSecureHandshakeServerEnabled 由 nps 启动时根据 bridge_secure_handshake 设置。
// 默认 false：服务端一律 legacy，不影响旧 npc。
func SetSecureHandshakeServerEnabled(enabled bool) {
	secureHandshakeServerEnabled.Store(enabled)
}

// SecureHandshakeServerEnabled 服务端是否允许向达标客户端发起挑战握手。
func SecureHandshakeServerEnabled() bool {
	return secureHandshakeServerEnabled.Load()
}

// SupportsLegacyHandshake 未达门槛或无法解析版本 → 旧握手。
func SupportsLegacyHandshake(clientRelease string) bool {
	if SecureHandshakeMinVersion == "" {
		return true
	}
	cmp, err := CompareDotVersion(clientRelease, SecureHandshakeMinVersion)
	if err != nil {
		return true
	}
	return cmp < 0
}

// UsesSecureHandshake 客户端/服务端共用：产品版本是否达标。
// 服务端在选路径时还需叠加 SecureHandshakeServerEnabled()。
func UsesSecureHandshake(clientRelease string) bool {
	if SecureHandshakeMinVersion == "" {
		return false
	}
	return !SupportsLegacyHandshake(clientRelease)
}

// ServerSelectsSecureHandshake 服务端实际是否对该客户端走挑战握手。
func ServerSelectsSecureHandshake(clientRelease string) bool {
	return SecureHandshakeServerEnabled() && UsesSecureHandshake(clientRelease)
}

// SupportsRemoteUpdate 报告客户端产品版本是否可接收远程升级信令。
// 空版本或无法解析 → false（默认安全，不向旧 npc 发未知帧）。
func SupportsRemoteUpdate(clientRelease string) bool {
	clientRelease = strings.TrimSpace(clientRelease)
	if clientRelease == "" || RemoteUpdateMinVersion == "" {
		return false
	}
	cmp, err := CompareDotVersion(clientRelease, RemoteUpdateMinVersion)
	if err != nil {
		return false
	}
	return cmp >= 0
}

// CompareDotVersion 比较 a、b（如 0.26.40）；a<b 返回 -1，相等 0，a>b 返回 1。
func CompareDotVersion(a, b string) (int, error) {
	left, err := parseDotVersion(a)
	if err != nil {
		return 0, err
	}
	right, err := parseDotVersion(b)
	if err != nil {
		return 0, err
	}
	n := len(left)
	if len(right) > n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(left) {
			x = left[i]
		}
		if i < len(right) {
			y = right[i]
		}
		if x < y {
			return -1, nil
		}
		if x > y {
			return 1, nil
		}
	}
	return 0, nil
}

func parseDotVersion(raw string) ([]int, error) {
	value := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	// 去掉旧字母后缀（如 0.26.40-fusion.1 → 0.26.40）；纯数字四段版如 0.26.41.1 原样比较
	if i := strings.IndexAny(value, "-+"); i >= 0 {
		value = value[:i]
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, strconv.ErrSyntax
	}
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}
