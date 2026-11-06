//go:build !npcgui
// +build !npcgui

package proxy

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/connection"
	"ehang.io/nps/server/tool"
	"ehang.io/nps/web"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type TunnelModeServer struct {
	BaseServer
	process    process
	listener   net.Listener
	listenerMu sync.RWMutex
	closed     bool
	virtual    map[net.Conn]struct{}
}

// tcp|http|host|mixProxy
func NewTunnelModeServer(process process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	return s
}

// 开始
func (s *TunnelModeServer) Start() error {
	s.listenerMu.RLock()
	closed := s.closed
	s.listenerMu.RUnlock()
	if closed {
		return net.ErrClosed
	}
	return conn.NewTcpListenerAndProcess(s.task.ServerIp+":"+strconv.Itoa(s.task.Port), func(c net.Conn) {
		if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
			logs.Warn("client id %d, task id %d,error %s, when tcp connection", s.task.Client.Id, s.task.Id, err.Error())
			c.Close()
			return
		}
		logs.Trace("new tcp connection,local port %d,client %d,remote address %s", s.task.Port, s.task.Client.Id, c.RemoteAddr())
		tool.IdleAutoStopInc(s.task.Id)
		s.process(conn.NewConn(c), s)
		tool.IdleAutoStopDec(s.task.Id)
		s.task.Client.AddConn()
	}, &s.listener)
}

// close
func (s *TunnelModeServer) Close() error {
	if s == nil {
		return nil
	}
	s.listenerMu.Lock()
	s.closed = true
	listener := s.listener
	s.listener = nil
	virtual := make([]net.Conn, 0, len(s.virtual))
	for c := range s.virtual {
		virtual = append(virtual, c)
		delete(s.virtual, c)
	}
	s.listenerMu.Unlock()
	for _, c := range virtual {
		_ = c.Close()
	}
	if listener == nil {
		return nil
	}
	return listener.Close()
}

// DialVirtual 将连接注入本隧道处理路径，不另开公网端口（tunnel://）。
func (s *TunnelModeServer) DialVirtual(remote string) (net.Conn, error) {
	if s == nil || s.task == nil || s.process == nil {
		return nil, errors.New("target tunnel is not configured")
	}
	remote = strings.TrimSpace(remote)
	remoteAddr, err := net.ResolveTCPAddr("tcp", remote)
	if err != nil {
		return nil, errors.New("invalid virtual visitor address")
	}
	clientConn, serverConn := net.Pipe()
	s.listenerMu.Lock()
	listener := s.listener
	if s.closed || listener == nil {
		s.listenerMu.Unlock()
		_ = clientConn.Close()
		_ = serverConn.Close()
		return nil, errors.New("target tunnel is not running")
	}
	if s.virtual == nil {
		s.virtual = make(map[net.Conn]struct{})
	}
	s.virtual[serverConn] = struct{}{}
	s.listenerMu.Unlock()

	wrapped := &virtualAddressConn{Conn: serverConn, local: listener.Addr(), remote: remoteAddr}
	go func() {
		defer func() {
			s.listenerMu.Lock()
			delete(s.virtual, serverConn)
			s.listenerMu.Unlock()
			_ = serverConn.Close()
		}()
		if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
			logs.Warn("virtual dial client id %d task id %d: %v", s.task.Client.Id, s.task.Id, err)
			return
		}
		tool.IdleAutoStopInc(s.task.Id)
		s.process(conn.NewConn(wrapped), s)
		tool.IdleAutoStopDec(s.task.Id)
		s.task.Client.AddConn()
	}()
	return clientConn, nil
}

type virtualAddressConn struct {
	net.Conn
	local  net.Addr
	remote net.Addr
}

func (c *virtualAddressConn) LocalAddr() net.Addr  { return c.local }
func (c *virtualAddressConn) RemoteAddr() net.Addr { return c.remote }

// web管理方式
type WebServer struct {
	BaseServer
	lifecycleMu   sync.Mutex
	server        *http.Server
	virtualServer *http.Server
	listener      net.Listener
	virtual       *tool.VirtualListener
	closed        bool
}

// 开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("web_port")
	if p == 0 {
		// 仍启动进程内虚拟入口，供 bridge://web 使用
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	// 空闲超时默认 2 小时（GC + Cookie 寿命对齐）；仅影响浏览器会话，与 npc 无关。
	if beego.BConfig.WebConfig.Session.SessionGCMaxLifetime <= 0 {
		beego.BConfig.WebConfig.Session.SessionGCMaxLifetime = 7200
	}
	if beego.BConfig.WebConfig.Session.SessionCookieLifeTime <= 0 {
		beego.BConfig.WebConfig.Session.SessionCookieLifeTime = int(beego.BConfig.WebConfig.Session.SessionGCMaxLifetime)
	}
	web.InitBeegoAssets()
	beego.InitBeforeHTTPRun()
	// Session 过滤器在 InitBeforeHTTPRun 中注册；安全入口门闸须紧随其后，才能读到 CruSession。
	web.RegisterSecureEntryFilter()
	// 管理面板 WAF（waf_enable + waf_protect_panel）；与域名反代共用规则。
	web.RegisterWAFPanelFilter()
	webSSL := beego.AppConfig.String("web_open_ssl") == "true"
	handler := newSameSiteHandler(beego.BeeApp.Handlers, webSSL)
	virtual := tool.NewVirtualListener(nil)
	virtualServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}

	var l net.Listener
	var err error
	if p > 0 {
		l, err = connection.GetWebManagerListener()
		if err != nil {
			_ = virtual.Close()
			logs.Error(err)
			return err
		}
	}

	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		_ = virtual.Close()
		if l != nil {
			_ = l.Close()
		}
		return net.ErrClosed
	}
	s.listener = l
	s.virtual = virtual
	s.virtualServer = virtualServer
	tool.SetWebServerListener(virtual)
	s.lifecycleMu.Unlock()

	go func() {
		if serveErr := virtualServer.Serve(virtual); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, net.ErrClosed) {
			logs.Error("in-process Web management listener stopped: %v", serveErr)
		}
	}()

	if l == nil {
		logs.Info("web_port=0: 管理面板仅可通过 bridge://web 进程内路由访问")
		stop := make(chan struct{})
		<-stop
		return nil
	}

	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	s.lifecycleMu.Lock()
	s.server = srv
	s.lifecycleMu.Unlock()

	if webSSL {
		keyPath := beego.AppConfig.String("web_key_file")
		certPath := beego.AppConfig.String("web_cert_file")
		if !filepath.IsAbs(certPath) {
			certPath = filepath.Join(common.GetRunPath(), certPath)
		}
		if !filepath.IsAbs(keyPath) {
			keyPath = filepath.Join(common.GetRunPath(), keyPath)
		}
		err = srv.ServeTLS(l, certPath, keyPath)
	} else {
		err = srv.Serve(l)
	}
	return err
}

func (s *WebServer) Close() error {
	if s == nil {
		return nil
	}
	s.lifecycleMu.Lock()
	s.closed = true
	srv := s.server
	vSrv := s.virtualServer
	virtual := s.virtual
	listener := s.listener
	s.server, s.virtualServer, s.virtual, s.listener = nil, nil, nil, nil
	s.lifecycleMu.Unlock()
	tool.SetWebServerListener(nil)
	if vSrv != nil {
		_ = vSrv.Close()
	}
	if virtual != nil {
		_ = virtual.Close()
	}
	if srv != nil {
		_ = srv.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	return nil
}

// new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

type process func(c *conn.Conn, s *TunnelModeServer) error

// tcp proxy
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("tcp port %d ,client id %d,task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}
	return s.DealClient(c, s.task.Client, targetAddr, nil, common.CONN_TCP, nil, s.task.Client.Flow, s.task.Target.LocalProxy, s.task, nil)
}

// http proxy
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	_, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		logs.Info(err)
		return err
	}
	// Authenticate before establishing CONNECT tunnel (fixes auth-after-200 bug).
	if err := s.proxyAuth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P); err != nil {
		logs.Warn("http proxy auth failed, client %d, remote %s: %v", s.task.Client.Id, c.Conn.RemoteAddr(), err)
		return err
	}
	if r.Method == "CONNECT" {
		if _, err := c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
			c.Close()
			return err
		}
		rb = nil
	}
	logs.Info("http proxy request, method %s, host %s, client %d, remote %s", r.Method, addr, s.task.Client.Id, c.Conn.RemoteAddr())
	return s.DealClient(c, s.task.Client, addr, rb, common.CONN_TCP, nil, s.task.Client.Flow, s.task.Target.LocalProxy, nil, nil)
}
