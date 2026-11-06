package server

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"ehang.io/nps/lib/file"
)

// TopologySnapshot 精简拓扑快照：不含密钥/证书路径。
type TopologySnapshot struct {
	Clients   []TopologyClient   `json:"clients"`
	Resources []TopologyResource `json:"resources"`
	UpdatedAt string             `json:"updatedAt"`
}

type TopologyClient struct {
	ID           int    `json:"id"`
	Remark       string `json:"remark"`
	Address      string `json:"address"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	Online       bool   `json:"online"`
	Connections  int32  `json:"connections"`
	FlowIn       int64  `json:"flowIn"`
	FlowOut      int64  `json:"flowOut"`
	LastOnlineAt string `json:"lastOnlineAt"`
}

type TopologyResource struct {
	ID       int    `json:"id"`
	Kind     string `json:"kind"`
	Mode     string `json:"mode"`
	ClientID int    `json:"clientId"`
	Remark   string `json:"remark"`
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	State    string `json:"state"`
	Entry    string `json:"entry"`
	Target   string `json:"target"`
	FlowIn   int64  `json:"flowIn"`
	FlowOut  int64  `json:"flowOut"`
}

// GetTopologyData 返回运维向资源概览；allowedClientIDs 非 nil 时按归属过滤（管理员传 nil）。
func GetTopologyData(allowedClientIDs map[int]struct{}, isAdmin bool) TopologySnapshot {
	if !isAdmin && allowedClientIDs == nil {
		allowedClientIDs = map[int]struct{}{}
	}
	dealClientData()

	clients := make(map[int]*file.Client)
	file.GetDb().JsonDb.Clients.Range(func(_, value interface{}) bool {
		client, ok := value.(*file.Client)
		if !ok || client == nil || client.NoDisplay {
			return true
		}
		if allowedClientIDs != nil {
			if _, ok := allowedClientIDs[client.Id]; !ok {
				return true
			}
		}
		clients[client.Id] = client
		return true
	})

	snapshot := TopologySnapshot{
		Clients:   make([]TopologyClient, 0, len(clients)),
		Resources: make([]TopologyResource, 0),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, client := range clients {
		in, out := int64(0), int64(0)
		if client.Flow != nil {
			in, out = client.Flow.InletFlow, client.Flow.ExportFlow
		}
		snapshot.Clients = append(snapshot.Clients, TopologyClient{
			ID: client.Id, Remark: client.Remark, Address: client.Addr, Version: client.Version,
			Enabled: client.Status, Online: client.IsConnect, Connections: client.NowConn,
			FlowIn: in, FlowOut: out, LastOnlineAt: client.LastOnlineTime,
		})
	}
	sort.Slice(snapshot.Clients, func(i, j int) bool { return snapshot.Clients[i].ID < snapshot.Clients[j].ID })

	file.GetDb().JsonDb.Tasks.Range(func(_, value interface{}) bool {
		task, ok := value.(*file.Tunnel)
		if !ok || task == nil || task.Client == nil {
			return true
		}
		clientID := task.Client.Id
		if _, ok := clients[clientID]; !ok {
			return true
		}
		online := clients[clientID].IsConnect
		enabled := task.Status
		running := enabled && online && task.RunStatus
		target := ""
		if task.Target != nil {
			target = task.Target.TargetStr
		}
		entry := task.TargetAddr
		if task.Port > 0 {
			entry = netJoinPort(task.ServerIp, task.Port)
		}
		in, out := int64(0), int64(0)
		if task.Flow != nil {
			in, out = task.Flow.InletFlow, task.Flow.ExportFlow
		}
		snapshot.Resources = append(snapshot.Resources, TopologyResource{
			ID: task.Id, Kind: "tunnel", Mode: task.Mode, ClientID: clientID, Remark: task.Remark,
			Enabled: enabled, Running: running, State: topologyState(enabled, running, online),
			Entry: entry, Target: target, FlowIn: in, FlowOut: out,
		})
		return true
	})
	file.GetDb().JsonDb.Hosts.Range(func(_, value interface{}) bool {
		host, ok := value.(*file.Host)
		if !ok || host == nil || host.Client == nil {
			return true
		}
		clientID := host.Client.Id
		if _, ok := clients[clientID]; !ok {
			return true
		}
		online := clients[clientID].IsConnect
		enabled := !host.IsClose
		running := enabled && online
		target := ""
		if host.Target != nil {
			target = host.Target.TargetStr
		}
		in, out := int64(0), int64(0)
		if host.Flow != nil {
			in, out = host.Flow.InletFlow, host.Flow.ExportFlow
		}
		snapshot.Resources = append(snapshot.Resources, TopologyResource{
			ID: host.Id, Kind: "host", Mode: "host", ClientID: clientID, Remark: host.Remark,
			Enabled: enabled, Running: running, State: topologyState(enabled, running, online),
			Entry: host.Host, Target: target, FlowIn: in, FlowOut: out,
		})
		return true
	})
	sort.Slice(snapshot.Resources, func(i, j int) bool {
		if snapshot.Resources[i].ClientID != snapshot.Resources[j].ClientID {
			return snapshot.Resources[i].ClientID < snapshot.Resources[j].ClientID
		}
		if snapshot.Resources[i].Kind != snapshot.Resources[j].Kind {
			return snapshot.Resources[i].Kind < snapshot.Resources[j].Kind
		}
		return snapshot.Resources[i].ID < snapshot.Resources[j].ID
	})
	return snapshot
}

func topologyState(enabled, running, online bool) string {
	if !enabled {
		return "disabled"
	}
	if running {
		return "running"
	}
	if !online {
		return "offline"
	}
	return "stopped"
}

func netJoinPort(ip string, port int) string {
	if ip == "" {
		ip = "0.0.0.0"
	}
	return fmt.Sprintf("%s:%s", ip, strconv.Itoa(port))
}
