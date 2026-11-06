package conn

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"ehang.io/nps/lib/crypt"
)

func TestBridgeQuicDialAcceptRoundTrip(t *testing.T) {
	crypt.InitTls()
	lnUDP, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := lnUDP.LocalAddr().String()
	_ = lnUDP.Close()

	tlsSrv := &tls.Config{
		Certificates: []tls.Certificate{crypt.GetCert()},
		NextProtos:   []string{BridgeQUICALPN},
		MinVersion:   tls.VersionTLS13,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	got := make(chan []byte, 1)
	go func() {
		_ = NewQuicListenerAndProcess(addr, tlsSrv, DefaultBridgeQuicConfig(), func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			buf := make([]byte, 64)
			n, rerr := io.ReadFull(c, buf[:4])
			if rerr != nil {
				t.Errorf("server read: %v", rerr)
				return
			}
			got <- append([]byte(nil), buf[:n]...)
			if _, werr := c.Write([]byte("pong")); werr != nil {
				t.Errorf("server write: %v", werr)
			}
			// 等客户端读完再关，避免 AutoClose 过早拆会话
			time.Sleep(200 * time.Millisecond)
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	var cli net.Conn
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		cli, err = DialQuic(ctx, addr, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{BridgeQUICALPN},
			MinVersion:         tls.VersionTLS13,
		}, DefaultBridgeQuicConfig())
		cancel()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer cli.Close()

	if _, err := cli.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	select {
	case b := <-got:
		if string(b) != "ping" {
			t.Fatalf("server got %q", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting server read")
	}
	_ = cli.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 8)
	n, err := io.ReadFull(cli, buf[:4])
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(buf[:n]) != "pong" {
		t.Fatalf("client got %q", buf[:n])
	}
	wg.Wait()
}
