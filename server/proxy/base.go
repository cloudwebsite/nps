package proxy

import (
	"errors"
	"net"
	"net/http"
	"sync"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego/logs"
)

type Service interface {
	Start() error
	Close() error
}

type NetBridge interface {
	SendLinkInfo(clientId int, link *conn.Link, t *file.Tunnel) (target net.Conn, err error)
}

// BaseServer struct
type BaseServer struct {
	id           int
	bridge       NetBridge
	task         *file.Tunnel
	errorContent []byte
	errorCode    int
	sync.Mutex
}

func NewBaseServer(bridge *bridge.Bridge, task *file.Tunnel) *BaseServer {
	return &BaseServer{
		bridge:       bridge,
		task:         task,
		errorContent: nil,
		Mutex:        sync.Mutex{},
	}
}

// add the flow
func (s *BaseServer) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.task.Flow.ExportFlow += out
	s.task.Flow.InletFlow += in
}

// change the flow
func (s *BaseServer) FlowAddHost(host *file.Host, in, out int64) {
	s.Lock()
	defer s.Unlock()
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

	// write fail bytes to the connection
	func (s *BaseServer) writeConnFail(c net.Conn) {
		s.writeConnFailInfo(c, "", "Cannot connect to the server", "")
	}

	// writeConnFailInfo 写出错误页并异步触发 error_page 通知。
	func (s *BaseServer) writeConnFailInfo(c net.Conn, host, message, remoteAddr string) {
		if message == "" {
			message = "Cannot connect to the server"
		}
		body := RenderErrorPage(s.errorContent, "502", host, message)
		_, _ = c.Write([]byte(common.ConnectionFailBytes))
		_, _ = c.Write(body)
		if remoteAddr == "" && c != nil && c.RemoteAddr() != nil {
			remoteAddr = c.RemoteAddr().String()
		}
		notifyErrorPageHit(host, message, remoteAddr)
	}

// auth check for reverse-proxy hosts (401 + WWW-Authenticate).
func (s *BaseServer) auth(r *http.Request, c *conn.Conn, u, p string) error {
	return s.doAuth(r, c, u, p, common.UnauthorizedBytes, "401 Unauthorized")
}

// proxyAuth check for HTTP forward proxy (407 + Proxy-Authenticate).
func (s *BaseServer) proxyAuth(r *http.Request, c *conn.Conn, u, p string) error {
	return s.doAuth(r, c, u, p, common.ProxyAuthRequiredBytes, "407 Proxy Authentication Required")
}

func (s *BaseServer) doAuth(r *http.Request, c *conn.Conn, u, p, failBytes, errMsg string) error {
	if u != "" && p != "" && !common.CheckAuth(r, u, p) {
		c.Write([]byte(failBytes))
		c.Close()
		return errors.New(errMsg)
	}
	return nil
}

	// check flow limit of the client ,and decrease the allow num of client
	func (s *BaseServer) CheckFlowAndConnNum(client *file.Client) error {
		if client.Flow.FlowLimit > 0 && (client.Flow.FlowLimit<<20) < (client.Flow.ExportFlow+client.Flow.InletFlow) {
			return errors.New("Traffic exceeded")
		}
		// 全局连接上限（默认 0=不限）
		if err := file.CheckGlobalConnLimit(); err != nil {
			return err
		}
		if !client.GetConn() {
			return errors.New("Connections exceed the current client limit")
		}
		return nil
	}

// create a new connection and start bytes copying
func (s *BaseServer) DealClient(c *conn.Conn, client *file.Client, addr string,
	rb []byte, tp string, f func(), flow *file.Flow, localProxy bool, task *file.Tunnel, host *file.Host) error {

		remote := c.RemoteAddr().String()
		// 三层源地址 ACL（全局 / 客户端 / 隧道·域名）；支持 geoip:/geosite:
		hostName := ""
		if host != nil {
			hostName = host.Host
		}
		if SourceDenied(remote, client, task, host, hostName) {
			c.Close()
			return nil
		}
		// WAF（TCP 等；与 ACL 并列，管理面板不经 BaseServer）
		if checkTCPWAF(remote) {
			c.Close()
			return nil
		}

		protoVersion := ""
		if task != nil {
			protoVersion = task.ProtoVersion
		}
		// 域名级 ProxyProtocol（隧道 ProtoVersion 优先，空时回落到 Host.Target）
		if protoVersion == "" {
			protoVersion = hostProxyProtocolVersion(host)
		}

		link := conn.NewLink(tp, addr, client.Cnf.Crypt, client.Cnf.Compress, c.Conn.RemoteAddr().String(), localProxy, protoVersion)
	if target, err := s.bridge.SendLinkInfo(client.Id, link, s.task); err != nil {
		logs.Warn("get connection from client id %d  error %s", client.Id, err.Error())
		c.Close()
		return err
	} else {
		if f != nil {
			f()
		}
		conn.CopyWaitGroup(target, c.Conn, link.Crypt, link.Compress, client.Rate, flow, true, rb, task, host)
	}
	return nil
}

	// IsGlobalBlackIp 仅检查全局层 ACL（兼容旧调用点）；空名单不限制。
	func IsGlobalBlackIp(ipPort string) bool {
		return SourceDenied(ipPort, nil, nil, nil, "")
	}
