package file

import (
	"fmt"
	"sync/atomic"
)

// 全局资源上限；配置为 0 表示不限制（默认，对齐现网）。

func MaxClients() int {
	return GetDb().GetGlobalSettings().MaxClients
}

func MaxTunnelsPerClient() int {
	return GetDb().GetGlobalSettings().MaxTunnelsPerClient
}

func MaxGlobalConn() int {
	return GetDb().GetGlobalSettings().MaxGlobalConn
}

// CountPersistedClients 统计可展示的客户端数（不含公共 NoDisplay）。
func CountPersistedClients() int {
	n := 0
	GetDb().JsonDb.Clients.Range(func(_, value interface{}) bool {
		c, ok := value.(*Client)
		if ok && c != nil && !c.NoDisplay {
			n++
		}
		return true
	})
	return n
}

// CountGlobalConnections 当前已占用连接数之和。
func CountGlobalConnections() int {
	var total int64
	GetDb().JsonDb.Clients.Range(func(_, value interface{}) bool {
		c, ok := value.(*Client)
		if ok && c != nil {
			total += int64(atomic.LoadInt32(&c.NowConn))
		}
		return true
	})
	return int(total)
}

// EffectiveClientTunnelLimit 客户端隧道上限：自身 MaxTunnelNum 优先，否则用全局默认。
func EffectiveClientTunnelLimit(client *Client) int {
	if client != nil && client.MaxTunnelNum > 0 {
		return client.MaxTunnelNum
	}
	return MaxTunnelsPerClient()
}

// CheckGlobalClientLimit 新建持久化客户端前检查。
func CheckGlobalClientLimit(c *Client) error {
	if c == nil || c.NoStore || c.NoDisplay {
		return nil
	}
	limit := MaxClients()
	if limit <= 0 {
		return nil
	}
	if CountPersistedClients() >= limit {
		return fmt.Errorf("max_clients limit reached (%d)", limit)
	}
	return nil
}

// CheckClientTunnelLimit 新建隧道/域名前检查有效上限。
func CheckClientTunnelLimit(client *Client) error {
	limit := EffectiveClientTunnelLimit(client)
	if limit <= 0 || client == nil {
		return nil
	}
	if client.GetTunnelNum() >= limit {
		return fmt.Errorf("max_tunnels_per_client limit reached (%d)", limit)
	}
	return nil
}

// CheckGlobalConnLimit 全局连接上限（在占用客户端连接槽之前）。
func CheckGlobalConnLimit() error {
	limit := MaxGlobalConn()
	if limit <= 0 {
		return nil
	}
	if CountGlobalConnections() >= limit {
		return fmt.Errorf("max_global_conn limit reached (%d)", limit)
	}
	return nil
}
