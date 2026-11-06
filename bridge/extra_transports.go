package bridge

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/gorilla/websocket"
	"github.com/xtaci/kcp-go"
)

// startExtraBridgeTransports 启动可选旁路监听。
// 未配置端口则跳过；失败返回 error，由调用方决定是否中止启动。
// 不改变既有 bridge_port / tls_bridge_port 语义。
func (s *Bridge) startExtraBridgeTransports() error {
	if s == nil {
		return nil
	}
	bridgeIP := strings.TrimSpace(beego.AppConfig.String("bridge_ip"))
	if bridgeIP == "" {
		bridgeIP = "0.0.0.0"
	}

	var closers []func() error

	rollback := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i]()
		}
	}

	// 旁路 KCP：主桥为 tcp 时，可用 bridge_kcp_port 额外开 KCP
	kcpPort := beego.AppConfig.DefaultInt("bridge_kcp_port", 0)
	if kcpPort > 0 && !(s.tunnelType == "kcp" && kcpPort == s.TunnelPort) {
		addr := net.JoinHostPort(bridgeIP, strconv.Itoa(kcpPort))
		ln, err := kcp.ListenWithOptions(addr, nil, 150, 3)
		if err != nil {
			return fmt.Errorf("start KCP Bridge on %s: %w", addr, err)
		}
		closers = append(closers, ln.Close)
		logs.Info("extra bridge: kcp listening on %s", addr)
		go func() {
			for {
				c, acceptErr := ln.AcceptKCP()
				if acceptErr != nil {
					logs.Warn("kcp bridge accept: %v", acceptErr)
					return
				}
				conn.SetUdpSession(c)
				go s.cliProcess(conn.NewConn(c))
			}
		}()
	}

	for _, endpoint := range []struct {
		key    string
		secure bool
	}{
		{key: "bridge_ws_port", secure: false},
		{key: "bridge_wss_port", secure: true},
	} {
		port := beego.AppConfig.DefaultInt(endpoint.key, 0)
		if port <= 0 {
			continue
		}
		prepared, err := s.prepareWebSocketBridge(bridgeIP, port, endpoint.secure)
		if err != nil {
			rollback()
			return err
		}
		closers = append(closers, prepared.listener.Close)
		go serveWebSocketBridge(prepared)
	}

		quicPort := beego.AppConfig.DefaultInt("bridge_quic_port", 0)
		if quicPort > 0 {
			addr := net.JoinHostPort(bridgeIP, strconv.Itoa(quicPort))
			tlsCfg := &tls.Config{
				Certificates: []tls.Certificate{crypt.GetCert()},
				NextProtos:   []string{conn.BridgeQUICALPN},
				MinVersion:   tls.VersionTLS13,
			}
			quicCfg := conn.DefaultBridgeQuicConfig()
			logs.Info("extra bridge: quic listening on %s alpn=%s", addr, conn.BridgeQUICALPN)
			go func() {
				if err := conn.NewQuicListenerAndProcess(addr, tlsCfg, quicCfg, func(c net.Conn) {
					s.cliProcess(conn.NewConn(c))
				}); err != nil {
					logs.Warn("quic bridge stopped: %v", err)
				}
			}()
		}

		return nil
	}

type preparedWebSocketBridge struct {
	server    *http.Server
	listener  net.Listener
	port      int
	path      string
	transport string
}

func (s *Bridge) prepareWebSocketBridge(ip string, port int, secure bool) (preparedWebSocketBridge, error) {
	path := strings.TrimSpace(beego.AppConfig.String("bridge_path"))
	if path == "" {
		path = "/ws"
	}
	parsedPath, err := url.ParseRequestURI(path)
	if err != nil || !strings.HasPrefix(path, "/") || parsedPath.RawQuery != "" || parsedPath.Fragment != "" {
		return preparedWebSocketBridge{}, fmt.Errorf("invalid bridge_path %q: expected an absolute URL path", path)
	}
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return preparedWebSocketBridge{}, fmt.Errorf("listen WebSocket Bridge on %s: %w", addr, err)
	}
	transport := "ws"
	if secure {
		transport = "wss"
		listener = tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{crypt.GetCert()},
			MinVersion:   tls.VersionTLS12,
		})
	}
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}
	bridge := s
	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    16 << 10,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != path {
				http.NotFound(w, r)
				return
			}
			ws, upgradeErr := upgrader.Upgrade(w, r, nil)
			if upgradeErr != nil {
				logs.Trace("[Bridge] %s WebSocket upgrade rejected: %v", transport, upgradeErr)
				return
			}
			stream, streamErr := conn.NewWebSocketStream(ws)
			if streamErr != nil {
				_ = ws.Close()
				return
			}
			go bridge.cliProcess(conn.NewConn(stream))
		}),
	}
	logs.Info("extra bridge: %s listening on %s path=%s", transport, addr, path)
	return preparedWebSocketBridge{
		server:    server,
		listener:  listener,
		port:      port,
		path:      path,
		transport: transport,
	}, nil
}

func serveWebSocketBridge(p preparedWebSocketBridge) {
	if err := p.server.Serve(p.listener); err != nil && err != http.ErrServerClosed {
		logs.Warn("%s bridge stopped: %v", p.transport, err)
	}
}

// ExtraBridgePortsConfigured 供测试/运维检查是否声明了旁路端口。
func ExtraBridgePortsConfigured() (kcp, ws, wss, quic int) {
	return beego.AppConfig.DefaultInt("bridge_kcp_port", 0),
		beego.AppConfig.DefaultInt("bridge_ws_port", 0),
		beego.AppConfig.DefaultInt("bridge_wss_port", 0),
		beego.AppConfig.DefaultInt("bridge_quic_port", 0)
}
