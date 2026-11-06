package conn

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	// BridgeQUICALPN Bridge QUIC 默认 ALPN（与 npc -type=quic 对齐）。
	BridgeQUICALPN = "nps"
	quicCloseDrain = 300 * time.Millisecond
	quicHandshake  = 10 * time.Second
)

// QuicStreamConn 将单条 QUIC stream 适配为 net.Conn（供 nps_mux / Bridge 握手）。
type QuicStreamConn struct {
	stream *quic.Stream
	sess   *quic.Conn
}

func NewQuicStreamConn(stream *quic.Stream, sess *quic.Conn) *QuicStreamConn {
	return &QuicStreamConn{stream: stream, sess: sess}
}

func (q *QuicStreamConn) Read(p []byte) (int, error)  { return q.stream.Read(p) }
func (q *QuicStreamConn) Write(p []byte) (int, error) { return q.stream.Write(p) }
func (q *QuicStreamConn) Close() error                { return q.stream.Close() }
func (q *QuicStreamConn) LocalAddr() net.Addr {
	if q.sess != nil {
		return q.sess.LocalAddr()
	}
	return nil
}
func (q *QuicStreamConn) RemoteAddr() net.Addr {
	if q.sess != nil {
		return q.sess.RemoteAddr()
	}
	return nil
}
func (q *QuicStreamConn) SetDeadline(t time.Time) error      { return q.stream.SetDeadline(t) }
func (q *QuicStreamConn) SetReadDeadline(t time.Time) error  { return q.stream.SetReadDeadline(t) }
func (q *QuicStreamConn) SetWriteDeadline(t time.Time) error { return q.stream.SetWriteDeadline(t) }

// QuicAutoCloseConn 关闭 stream 后延迟关闭会话，避免未确认数据被丢弃。
type QuicAutoCloseConn struct {
	*QuicStreamConn
	closeOnce sync.Once
}

func NewQuicAutoCloseConn(stream *quic.Stream, sess *quic.Conn) *QuicAutoCloseConn {
	return &QuicAutoCloseConn{QuicStreamConn: NewQuicStreamConn(stream, sess)}
}

func (q *QuicAutoCloseConn) Close() error {
	q.closeOnce.Do(func() {
		_ = q.QuicStreamConn.Close()
		go func() {
			t := time.NewTimer(quicCloseDrain)
			defer t.Stop()
			select {
			case <-q.sess.Context().Done():
			case <-t.C:
			}
			_ = q.sess.CloseWithError(0, "close")
		}()
	})
	return nil
}

// DialQuic 拨号到 Bridge QUIC 并打开首条 stream。
func DialQuic(ctx context.Context, addr string, tlsCfg *tls.Config, quicCfg *quic.Config) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if tlsCfg == nil {
		tlsCfg = &tls.Config{InsecureSkipVerify: true, NextProtos: []string{BridgeQUICALPN}, MinVersion: tls.VersionTLS13}
	}
	if len(tlsCfg.NextProtos) == 0 {
		cp := tlsCfg.Clone()
		cp.NextProtos = []string{BridgeQUICALPN}
		tlsCfg = cp
	}
	if quicCfg == nil {
		quicCfg = DefaultBridgeQuicConfig()
	}
	sess, err := quic.DialAddr(ctx, addr, tlsCfg, quicCfg)
	if err != nil {
		return nil, err
	}
	stream, err := sess.OpenStreamSync(ctx)
	if err != nil {
		_ = sess.CloseWithError(0, "open stream")
		return nil, err
	}
	return NewQuicAutoCloseConn(stream, sess), nil
}

// DefaultBridgeQuicConfig Bridge 侧默认 QUIC 参数。
func DefaultBridgeQuicConfig() *quic.Config {
	return &quic.Config{
		KeepAlivePeriod:    10 * time.Second,
		MaxIdleTimeout:     30 * time.Second,
		MaxIncomingStreams: 1024,
	}
}

// NewQuicListenerAndProcess 监听 QUIC；每条会话接受首条 stream 后回调。
func NewQuicListenerAndProcess(addr string, tlsConfig *tls.Config, quicConfig *quic.Config, f func(c net.Conn)) error {
	if tlsConfig == nil {
		return net.ErrClosed
	}
	if quicConfig == nil {
		quicConfig = DefaultBridgeQuicConfig()
	}
	ln, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return err
	}
	for {
		sess, err := ln.Accept(context.Background())
		if err != nil {
			_ = ln.Close()
			return err
		}
		go acceptQuicStream(sess, f)
	}
}

func acceptQuicStream(sess *quic.Conn, f func(c net.Conn)) {
	ctx, cancel := context.WithTimeout(sess.Context(), quicHandshake)
	stream, err := sess.AcceptStream(ctx)
	cancel()
	if err != nil {
		_ = sess.CloseWithError(0, "closed")
		return
	}
	f(NewQuicAutoCloseConn(stream, sess))
}
