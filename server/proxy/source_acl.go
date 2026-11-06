package proxy

import (
	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego/logs"
)

// SourceDenied 三层源地址 ACL：全局 → 客户端 → 隧道/域名；任一层拒绝即 true。
// hostName 供 geosite: 规则（HTTP 域名入口传入 Host；TCP 可空）。
func SourceDenied(ipPort string, client *file.Client, tunnel *file.Tunnel, host *file.Host, hostName string) bool {
	ip := common.GetIpByAddr(ipPort)
	if ip == "" {
		return false
	}
	global := file.GetDb().GetGlobal()
	gPol := acl.SourcePolicy{}
	if global != nil {
		gPol = acl.SourcePolicy{
			Priority: global.IpListPriority,
			Black:    global.BlackIpList,
			White:    global.WhiteIpList,
		}
	}
	cPol := acl.SourcePolicy{}
	if client != nil {
		cPol = acl.SourcePolicy{
			Priority: client.IpListPriority,
			Black:    client.BlackIpList,
			White:    client.IpWhiteList,
		}
	}
	ePol := acl.SourcePolicy{}
	if tunnel != nil {
		ePol = acl.SourcePolicy{
			Priority: tunnel.IpListPriority,
			Black:    tunnel.BlackIpList,
			White:    tunnel.WhiteIpList,
		}
	} else if host != nil {
		ePol = acl.SourcePolicy{
			Priority: host.IpListPriority,
			Black:    host.BlackIpList,
			White:    host.WhiteIpList,
		}
	}
	if acl.AllowsLevels(ip, hostName, gPol, cPol, ePol) {
		return false
	}
	logs.Warn("源地址 ACL 拒绝 ip=%s host=%s", ip, hostName)
	return true
}
