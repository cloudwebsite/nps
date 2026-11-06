// Package p2pholepunch 提供可选的 djylb holepunch 协调器入口。
// 默认不监听；p2p_holepunch_port>0 时启动。仅 modern npc 会主动拨入，旧客户端无感。
package p2pholepunch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/crypt"
	"github.com/astaxie/beego/logs"
	"github.com/djylb/holepunch/coord"
	"github.com/djylb/holepunch/natprobe"
	"github.com/djylb/holepunch/protocol"
	"github.com/djylb/holepunch/stun"
)

// Server 可选 holepunch 协调器。
type Server struct {
	mu       sync.Mutex
	listener net.Listener
	probe    *natprobe.Server
	co       *coord.Coordinator
	pending  map[string]*pairSlot // password -> waiting peer
	cancel   context.CancelFunc
}

type pairSlot struct {
	role string // visitor|provider
	conn net.Conn
	done chan struct{}
}

// Start 在 port 上启动 TLS 协调入口；basePort 为 natprobe UDP 基址（0=随机）。
func Start(port, basePort int) (*Server, error) {
	if port <= 0 || port > 65535 {
		return nil, errors.New("p2p_holepunch_port invalid")
	}
	cert := crypt.GetCert()
	if len(cert.Certificate) == 0 {
		return nil, errors.New("p2p holepunch: no bridge certificate")
	}
	reg := natprobe.NewRegistry(natprobe.RegistryConfig{})
	groups := []natprobe.ObserverGroup{
		{Bind: netip.IPv4Unspecified(), Slots: 3},
	}
	probe, err := natprobe.NewServer(natprobe.ServerConfig{
		Registry: reg,
		Groups:   groups,
		BasePort: basePort,
		Logger:   slog.Default(),
	})
	if err != nil {
		return nil, err
	}
	if err := probe.Start(); err != nil {
		_ = probe.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	stunList := natprobe.NewSTUNList(natprobe.STUNListConfig{Servers: stun.DefaultServers()})
	_ = stunList.Refresh(ctx)
	go stunList.Run(ctx)
	src := natprobe.SpecSource{Server: probe, STUN: stunList}
	co, err := coord.New(coord.Config{Registry: reg, Probe: coord.ProbeFrom(src)})
	if err != nil {
		cancel()
		_ = probe.Close()
		return nil, err
	}
	ln, err := tls.Listen("tcp", ":"+strconv.Itoa(port), &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		cancel()
		_ = co.Close()
		_ = probe.Close()
		return nil, err
	}
	s := &Server{
		listener: ln,
		probe:    probe,
		co:       co,
		pending:  make(map[string]*pairSlot),
		cancel:   cancel,
	}
	go s.serve()
	logs.Info("P2P holepunch 协调器已启动 port=%d（仅 modern 客户端拨入）", port)
	return s, nil
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	if s.co != nil {
		_ = s.co.Close()
	}
	if s.probe != nil {
		_ = s.probe.Close()
	}
	return nil
}

func (s *Server) serve() {
	for {
		c, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(c)
	}
}

// 首行：password\trole\n ；role=visitor|provider
func (s *Server) handle(c net.Conn) {
	_ = c.SetDeadline(time.Now().Add(15 * time.Second))
	line, err := readLine(c)
	if err != nil {
		_ = c.Close()
		return
	}
	parts := strings.SplitN(strings.TrimSpace(line), "\t", 2)
	if len(parts) != 2 {
		_ = c.Close()
		return
	}
	password, role := parts[0], strings.ToLower(parts[1])
	if password == "" || (role != "visitor" && role != "provider") {
		_ = c.Close()
		return
	}
	_ = c.SetDeadline(time.Time{})

	s.mu.Lock()
	slot := s.pending[password]
	if slot == nil {
		done := make(chan struct{})
		s.pending[password] = &pairSlot{role: role, conn: c, done: done}
		s.mu.Unlock()
		select {
		case <-time.After(30 * time.Second):
			s.mu.Lock()
			if cur := s.pending[password]; cur != nil && cur.conn == c {
				delete(s.pending, password)
				_ = c.Close()
			}
			s.mu.Unlock()
		case <-done:
			// 配对成功，runSession 由对端 goroutine 启动
		}
		return
	}
	if slot.role == role {
		s.mu.Unlock()
		_ = c.Close()
		return
	}
	peer := slot.conn
	delete(s.pending, password)
	close(slot.done)
	s.mu.Unlock()

	visitor, provider := c, peer
	if role == "provider" {
		visitor, provider = peer, c
	}
	s.runSession(visitor, provider)
}

func (s *Server) runSession(visitor, provider net.Conn) {
	sess, err := s.co.NewSession(coord.SessionOptions{})
	if err != nil {
		logs.Warn("holepunch NewSession: %v", err)
		_ = visitor.Close()
		_ = provider.Close()
		return
	}
	// Attach 成功后连接归 session；失败时仍归调用方关闭。
	if err := sess.Attach(protocol.RoleInitiator, visitor, coord.AttachOptions{}); err != nil {
		logs.Warn("holepunch attach visitor: %v", err)
		_ = visitor.Close()
		_ = provider.Close()
		return
	}
	if err := sess.Attach(protocol.RoleResponder, provider, coord.AttachOptions{}); err != nil {
		logs.Warn("holepunch attach provider: %v", err)
		_ = provider.Close()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := sess.Wait(ctx); err != nil {
		logs.Info("holepunch session ended: %v", err)
	}
}

func readLine(r io.Reader) (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return b.String(), nil
			}
			if buf[0] != '\r' {
				b.WriteByte(buf[0])
			}
			if b.Len() > 512 {
				return "", fmt.Errorf("line too long")
			}
		}
		if err != nil {
			return b.String(), err
		}
	}
}
