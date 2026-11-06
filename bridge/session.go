package bridge

import (
	"math/rand"

	"ehang.io/nps/lib/nps_mux"
)

// 同 VerifyKey 多 WORK_CHAN 会话选取。
// Primary：保持「后来者替换」的旧行为（单隧道）。
// RoundRobin / Random：保留多条隧道并按策略选一条转发。

func (c *Client) replaceTunnelLocked(mux *nps_mux.Mux) (old *nps_mux.Mux) {
	if c == nil {
		return nil
	}
	old = c.tunnel
	c.tunnel = mux
	c.tunnels = nil
	if mux != nil {
		c.tunnels = []*nps_mux.Mux{mux}
	}
	return old
}

func (c *Client) addTunnelLocked(mux *nps_mux.Mux) (toClose []*nps_mux.Mux) {
	if c == nil || mux == nil {
		return nil
	}
	mode := ClientSelectMode()
	if mode == Primary {
		old := c.replaceTunnelLocked(mux)
		if old != nil && old != mux {
			return []*nps_mux.Mux{old}
		}
		return nil
	}
	// 清理已关闭条目后追加
	alive := make([]*nps_mux.Mux, 0, len(c.tunnels)+1)
	for _, t := range c.tunnels {
		if t == nil || t.IsClose() {
			if t != nil && t != mux {
				toClose = append(toClose, t)
			}
			continue
		}
		if t == mux {
			continue
		}
		alive = append(alive, t)
	}
	alive = append(alive, mux)
	c.tunnels = alive
	c.tunnel = mux // 最新一条作状态展示
	return toClose
}

func (c *Client) pickTunnelLocked() *nps_mux.Mux {
	if c == nil {
		return nil
	}
	candidates := make([]*nps_mux.Mux, 0, len(c.tunnels)+1)
	seen := map[*nps_mux.Mux]struct{}{}
	add := func(m *nps_mux.Mux) {
		if m == nil || m.IsClose() {
			return
		}
		if _, ok := seen[m]; ok {
			return
		}
		seen[m] = struct{}{}
		candidates = append(candidates, m)
	}
	for _, m := range c.tunnels {
		add(m)
	}
	add(c.tunnel)
	if len(candidates) == 0 {
		return nil
	}
	switch ClientSelectMode() {
	case Random:
		return candidates[rand.Intn(len(candidates))]
	case RoundRobin:
		idx := c.selectSeq.Add(1) - 1
		return candidates[idx%uint64(len(candidates))]
	default:
		// Primary：优先最新（tunnel 字段）
		if c.tunnel != nil && !c.tunnel.IsClose() {
			return c.tunnel
		}
		return candidates[len(candidates)-1]
	}
}

func (c *Client) closeAllTunnelsLocked() []*nps_mux.Mux {
	if c == nil {
		return nil
	}
	out := make([]*nps_mux.Mux, 0, len(c.tunnels)+1)
	seen := map[*nps_mux.Mux]struct{}{}
	collect := func(m *nps_mux.Mux) {
		if m == nil {
			return
		}
		if _, ok := seen[m]; ok {
			return
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	for _, m := range c.tunnels {
		collect(m)
	}
	collect(c.tunnel)
	c.tunnel = nil
	c.tunnels = nil
	return out
}
