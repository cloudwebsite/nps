package main

import (
	"strings"
)

// npcConnection 描述一条 npc 控制面连接。
type npcConnection struct {
	server    string
	vkey      string
	transport string
	tls       bool
}

// expandNpcConnections 解析多 server / 多 type / 多 vkey。
// 短列表复用末项：`-server=a:1,b:2 -vkey=key -type=tcp,tls` 合法且确定。
// 单入口旧命令行为不变。
func expandNpcConnections(servers, vkeys, transports string, defaultTLS bool) []npcConnection {
	split := func(value string) []string {
		value = strings.NewReplacer("，", ",", "：", ":").Replace(value)
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			out = append(out, p)
		}
		return out
	}
	serverList := split(servers)
	vkeyList := split(vkeys)
	transportList := split(transports)
	if len(serverList) == 0 || len(vkeyList) == 0 {
		return nil
	}
	if len(transportList) == 0 {
		transportList = []string{"tcp"}
	}
	maxLength := len(serverList)
	if len(vkeyList) > maxLength {
		maxLength = len(vkeyList)
	}
	if len(transportList) > maxLength {
		maxLength = len(transportList)
	}
	valueAt := func(values []string, index int) string {
		if len(values) == 0 {
			return ""
		}
		if index < len(values) {
			return values[index]
		}
		return values[len(values)-1]
	}
	connections := make([]npcConnection, 0, maxLength)
	for index := 0; index < maxLength; index++ {
		transport := strings.ToLower(valueAt(transportList, index))
		tls := defaultTLS
		if transport == "tls" {
			transport = "tcp"
			tls = true
		}
		connections = append(connections, npcConnection{
			server:    valueAt(serverList, index),
			vkey:      valueAt(vkeyList, index),
			transport: transport,
			tls:       tls,
		})
	}
	return connections
}
