package tool

import "ehang.io/nps/lib/file"

// 便捷再导出（实现见 lib/file/limits.go）。

func MaxClients() int                 { return file.MaxClients() }
func MaxTunnelsPerClient() int        { return file.MaxTunnelsPerClient() }
func MaxGlobalConn() int              { return file.MaxGlobalConn() }
func CountPersistedClients() int      { return file.CountPersistedClients() }
func CountGlobalConnections() int     { return file.CountGlobalConnections() }
func CanAddClient() error             { return file.CheckGlobalClientLimit(&file.Client{}) }
func CanAcceptConnection() error      { return file.CheckGlobalConnLimit() }

func EffectiveClientTunnelLimit(client *file.Client) int {
	return file.EffectiveClientTunnelLimit(client)
}

func CanAddTunnelOrHost(client *file.Client) error {
	return file.CheckClientTunnelLimit(client)
}
