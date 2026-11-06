package p2pholepunch

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/djylb/holepunch"
)

// DialAndPunch 客户端侧：拨入协调器并 Punch（modern）。
func DialAndPunch(ctx context.Context, addr, password, role string) (*holepunch.Session, error) {
	if addr == "" || password == "" {
		return nil, errors.New("holepunch dial: empty addr or password")
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	raw, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
	})
	if err != nil {
		return nil, err
	}
	header := password + "\t" + role + "\n"
	if _, err := raw.Write([]byte(header)); err != nil {
		_ = raw.Close()
		return nil, err
	}
	sess, err := holepunch.Punch(ctx, raw, holepunch.Options{
		TransportMode: "quic",
	})
	_ = raw.Close() // Punch 成功后控制连接归调用方；协议帧已结束
	if err != nil {
		return nil, err
	}
	return sess, nil
}
