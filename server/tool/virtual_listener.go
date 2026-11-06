package tool

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

const virtualListenerQueueSize = 1024

// VirtualListener 进程内 Listener：不占公网端口，供 bridge://web 等嵌套入口使用。
type VirtualListener struct {
	addr  net.Addr
	queue chan net.Conn
	done  chan struct{}
	once  sync.Once
	mu    sync.Mutex
	close bool
}

type virtualAppConn struct {
	net.Conn
	local  net.Addr
	remote net.Addr
}

func (c *virtualAppConn) LocalAddr() net.Addr  { return c.local }
func (c *virtualAppConn) RemoteAddr() net.Addr { return c.remote }

func NewVirtualListener(addr net.Addr) *VirtualListener {
	if addr == nil {
		addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	}
	return &VirtualListener{
		addr:  addr,
		queue: make(chan net.Conn, virtualListenerQueueSize),
		done:  make(chan struct{}),
	}
}

func (l *VirtualListener) Accept() (net.Conn, error) {
	if l == nil {
		return nil, net.ErrClosed
	}
	select {
	case <-l.done:
		return nil, net.ErrClosed
	default:
	}
	select {
	case <-l.done:
		return nil, net.ErrClosed
	case c := <-l.queue:
		if c == nil {
			return nil, net.ErrClosed
		}
		return c, nil
	}
}

func (l *VirtualListener) Close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		l.mu.Lock()
		l.close = true
		close(l.done)
		for {
			select {
			case c := <-l.queue:
				if c != nil {
					_ = c.Close()
				}
			default:
				l.mu.Unlock()
				return
			}
		}
	})
	return nil
}

func (l *VirtualListener) Addr() net.Addr {
	if l == nil {
		return nil
	}
	return l.addr
}

// Dial 将调用方接到本 Listener；remote 可为 host:port，用于保留访客地址语义。
func (l *VirtualListener) Dial(ctx context.Context, remote string) (net.Conn, error) {
	if l == nil {
		return nil, net.ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	remote = strings.TrimSpace(remote)
	var remoteAddr net.Addr = &net.TCPAddr{}
	if remote != "" {
		resolved, err := net.ResolveTCPAddr("tcp", remote)
		if err != nil {
			return nil, err
		}
		remoteAddr = resolved
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	caller, accepted := net.Pipe()
	caller = &virtualAppConn{Conn: caller, local: remoteAddr, remote: l.addr}
	accepted = &virtualAppConn{Conn: accepted, local: l.addr, remote: remoteAddr}
	l.mu.Lock()
	if l.close {
		l.mu.Unlock()
		_ = caller.Close()
		_ = accepted.Close()
		return nil, net.ErrClosed
	}
	select {
	case l.queue <- accepted:
		l.mu.Unlock()
		return caller, nil
	default:
		l.mu.Unlock()
		_ = caller.Close()
		_ = accepted.Close()
		return nil, errors.New("virtual listener queue is full")
	}
}

var virtualApplicationListeners struct {
	sync.RWMutex
	web *VirtualListener
}

func SetWebServerListener(listener *VirtualListener) {
	virtualApplicationListeners.Lock()
	virtualApplicationListeners.web = listener
	virtualApplicationListeners.Unlock()
}

func GetWebServerConn(ctx context.Context, remote string) (net.Conn, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	virtualApplicationListeners.RLock()
	listener := virtualApplicationListeners.web
	virtualApplicationListeners.RUnlock()
	if listener == nil {
		return nil, errors.New("web server is not running")
	}
	return listener.Dial(ctx, remote)
}

// DialBridgeApp 解析 bridge:// 应用目标（仅 web）。
func DialBridgeApp(target, remote string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "web":
		return GetWebServerConn(ctx, remote)
	default:
		return nil, errors.New("unsupported bridge:// target (only web in this slice)")
	}
}
