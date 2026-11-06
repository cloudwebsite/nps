package proxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/quic-go/quic-go/http3"
)

// http3ListenerService 持有 UDP 与 quic-go HTTP/3 Server（默认不启动）。
type http3ListenerService struct {
	packet net.PacketConn
	h3     *http3.Server
	once   sync.Once
}

func (s *http3ListenerService) Close() error {
	if s == nil {
		return nil
	}
	var closeErr error
	s.once.Do(func() {
		if s.h3 != nil {
			if err := s.h3.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
				closeErr = err
			}
		}
		if s.packet != nil {
			if err := s.packet.Close(); err != nil && closeErr == nil && !errors.Is(err, net.ErrClosed) {
				closeErr = err
			}
		}
	})
	return closeErr
}

// startHTTP3 启动浏览器向的 HTTP/3 域名反代；未配置端口则不调用。
func (s *httpServer) startHTTP3() (net.PacketConn, http3Service, error) {
	if s == nil || s.http3Port <= 0 {
		return nil, nil, errors.New("HTTP/3 端口未配置")
	}
	ip := strings.TrimSpace(beego.AppConfig.String("http_proxy_ip"))
	if ip == "" {
		ip = "0.0.0.0"
	}
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ip, strconv.Itoa(s.http3Port)))
	if err != nil {
		return nil, nil, err
	}
	packetConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, nil, err
	}
	server := &http3.Server{Handler: http.HandlerFunc(s.handleHTTP3), TLSConfig: s.http3TLSConfig()}
	service := &http3ListenerService{packet: packetConn, h3: server}
	go func() {
		if serveErr := server.Serve(packetConn); serveErr != nil && !errors.Is(serveErr, net.ErrClosed) {
			logs.Error("HTTP/3 反代退出: %v", serveErr)
		}
	}()
	logs.Info("[HTTP/3] 域名反代监听 UDP %s", packetConn.LocalAddr())
	return packetConn, service, nil
}

func (s *httpServer) handleHTTP3(w http.ResponseWriter, r *http.Request) {
	if r == nil {
		return
	}
	r.URL.Scheme = "https"
	s.handleTunneling(w, r)
}

func (s *httpServer) http3TLSConfig() *tls.Config {
	config := &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{http3.NextProtoH3}}
	config.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if s != nil && s.acmeManager != nil {
			if cert, err := s.acmeManager.GetCertificate(hello); err == nil && cert != nil {
				return cert, nil
			}
		}
		name := ""
		if hello != nil {
			name = hello.ServerName
		}
		return loadHTTP3Certificate(name)
	}
	return config
}

func loadHTTP3Certificate(serverName string) (*tls.Certificate, error) {
	serverName = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(serverName)), ".")
	if serverName == "" {
		return nil, errors.New("HTTP/3 请求缺少 SNI")
	}
	r := &http.Request{Host: serverName, RequestURI: "/", URL: &url.URL{Scheme: "https", Path: "/"}}
	if host, err := file.GetDb().GetInfoByHost(serverName, r); err == nil && host != nil {
		host.RLock()
		certFile, keyFile := host.CertFilePath, host.KeyFilePath
		justProxy := host.HttpsJustProxy
		host.RUnlock()
		if justProxy {
			return nil, errors.New("当前 Host 使用 TLS 直通，不支持 HTTP/3 七层反代")
		}
		if cert, err := loadCertificateMaterial(certFile, keyFile); err == nil {
			return cert, nil
		}
	}
	defaultCert := strings.TrimSpace(beego.AppConfig.String("https_default_cert_file"))
	defaultKey := strings.TrimSpace(beego.AppConfig.String("https_default_key_file"))
	if defaultCert == "" {
		defaultCert = filepath.Join(common.GetRunPath(), "conf", "server.pem")
	}
	if defaultKey == "" {
		defaultKey = filepath.Join(common.GetRunPath(), "conf", "server.key")
	}
	return loadCertificateMaterial(defaultCert, defaultKey)
}

func loadCertificateMaterial(certFile, keyFile string) (*tls.Certificate, error) {
	certFile, keyFile = strings.TrimSpace(certFile), strings.TrimSpace(keyFile)
	if certFile == "" || keyFile == "" {
		return nil, errors.New("证书或私钥未配置")
	}
	var certPEM, keyPEM []byte
	if strings.Contains(certFile, "-----BEGIN") || strings.Contains(keyFile, "-----BEGIN") {
		certPEM, keyPEM = []byte(certFile), []byte(keyFile)
	} else {
		certFile = common.ResolveConfigPath(certFile)
		keyFile = common.ResolveConfigPath(keyFile)
		var err error
		certPEM, err = os.ReadFile(certFile)
		if err != nil {
			return nil, err
		}
		keyPEM, err = os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("加载证书失败: %w", err)
	}
	return &cert, nil
}
