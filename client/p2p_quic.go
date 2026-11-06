package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"ehang.io/nps/lib/crypt"
	"github.com/astaxie/beego/logs"
	"github.com/quic-go/quic-go"
)

const p2pQUICALPN = "nps-p2p"

// dialP2PQUIC 在已打通的 UDP PacketConn 上发起 QUIC（modern）。
// 不向 nps 控制面发新帧；仅两端均为 modern 时可用。
func dialP2PQUIC(packetConn net.PacketConn, remoteAddress string) (net.Conn, error) {
	if packetConn == nil {
		return nil, fmt.Errorf("p2p quic: packet conn is nil")
	}
	remote, err := net.ResolveUDPAddr("udp", remoteAddress)
	if err != nil {
		return nil, err
	}
	tr := &quic.Transport{Conn: packetConn}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	session, err := tr.Dial(ctx, remote, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{p2pQUICALPN},
		MinVersion:         tls.VersionTLS13,
	}, &quic.Config{MaxIncomingStreams: 1024, KeepAlivePeriod: 10 * time.Second})
	if err != nil {
		_ = tr.Close()
		return nil, err
	}
	stream, err := session.OpenStreamSync(ctx)
	if err != nil {
		_ = session.CloseWithError(0, "open stream")
		_ = tr.Close()
		return nil, err
	}
	logs.Info("P2P modern：QUIC 拨号成功 remote=%s", remoteAddress)
	return &p2pQUICConn{Stream: stream, session: session, transport: tr}, nil
}

// acceptP2PQUIC 在已打通的 UDP PacketConn 上接受 QUIC（provider 侧）。
func acceptP2PQUIC(packetConn net.PacketConn, expectRemote string) (net.Conn, error) {
	if packetConn == nil {
		return nil, fmt.Errorf("p2p quic: packet conn is nil")
	}
	tr := &quic.Transport{Conn: packetConn}
	ln, err := tr.Listen(&tls.Config{
		Certificates: []tls.Certificate{crypt.GetCert()},
		NextProtos:   []string{p2pQUICALPN},
		MinVersion:   tls.VersionTLS13,
	}, &quic.Config{MaxIncomingStreams: 1024, KeepAlivePeriod: 10 * time.Second})
	if err != nil {
		_ = tr.Close()
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	session, err := ln.Accept(ctx)
	if err != nil {
		_ = ln.Close()
		_ = tr.Close()
		return nil, err
	}
	if expectRemote != "" && session.RemoteAddr() != nil && session.RemoteAddr().String() != expectRemote {
		logs.Warn("P2P modern：QUIC 对端地址 %s 与期望 %s 不一致，仍继续", session.RemoteAddr(), expectRemote)
	}
	stream, err := session.AcceptStream(ctx)
	if err != nil {
		_ = session.CloseWithError(0, "accept stream")
		_ = ln.Close()
		_ = tr.Close()
		return nil, err
	}
	logs.Info("P2P modern：QUIC 接入成功 remote=%v", session.RemoteAddr())
	return &p2pQUICConn{Stream: stream, session: session, transport: tr, listener: ln}, nil
}

// p2pQUICConn 将首条 QUIC stream 作为 nps_mux 底层连接；关闭时释放会话与 Transport。
type p2pQUICConn struct {
	*quic.Stream
	session   *quic.Conn
	transport *quic.Transport
	listener  *quic.Listener
}

func (c *p2pQUICConn) Close() error {
	var first error
	if c.Stream != nil {
		first = c.Stream.Close()
	}
	if c.session != nil {
		_ = c.session.CloseWithError(0, "close")
	}
	if c.listener != nil {
		_ = c.listener.Close()
	}
	if c.transport != nil {
		_ = c.transport.Close()
	}
	return first
}

func (c *p2pQUICConn) LocalAddr() net.Addr {
	if c.session != nil {
		return c.session.LocalAddr()
	}
	return nil
}

func (c *p2pQUICConn) RemoteAddr() net.Addr {
	if c.session != nil {
		return c.session.RemoteAddr()
	}
	return nil
}
