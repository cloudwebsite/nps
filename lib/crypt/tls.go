package crypt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var (
	cert                tls.Certificate
	certLoadedFromFiles atomic.Bool
	clientFP            string // 客户端期望的服务端指纹；空 = 沿用 InsecureSkipVerify
)

func InitTls() {
	certLoadedFromFiles.Store(false)
	c, k, err := generateKeyPair("NPS Org")
	if err == nil {
		cert, err = tls.X509KeyPair(c, k)
	}
	if err != nil {
		log.Fatalln("Error initializing crypto certs", err)
	}
}

// InitTlsFromFiles 加载运维配置的 Bridge 证书；失败由调用方回退 InitTls。
func InitTlsFromFiles(certPath, keyPath string) error {
	certPath = strings.TrimSpace(certPath)
	keyPath = strings.TrimSpace(keyPath)
	if certPath == "" || keyPath == "" {
		return fmt.Errorf("Bridge certificate and key paths must both be configured")
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read Bridge certificate %q: %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read Bridge private key %q: %w", keyPath, err)
	}
	loaded, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("parse Bridge certificate: %w", err)
	}
	cert = loaded
	certLoadedFromFiles.Store(true)
	return nil
}

func GetCert() tls.Certificate {
	return cert
}

// GetCertFingerprint 返回当前 Bridge 叶子证书的 SHA-256 hex（运维展示 / 指纹固定参考）。
func GetCertFingerprint() string {
	if len(cert.Certificate) == 0 {
		return ""
	}
	sum := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sum[:])
}

// IsCertLoadedFromFiles 是否正在使用文件证书（相对临时自签名）。
func IsCertLoadedFromFiles() bool {
	return certLoadedFromFiles.Load()
}

func NewTlsServerConn(conn net.Conn) net.Conn {
	config := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	return tls.Server(conn, config)
}

// NewTlsClientConn 三态：
//   - 未设指纹：InsecureSkipVerify（旧行为）
//   - 已设指纹：InsecureSkipVerify + VerifyPeerCertificate 比对 SHA-256
func NewTlsClientConn(conn net.Conn) net.Conn {
	return tls.Client(conn, TlsDialConfig())
}

// SetTlsFingerprint 设置客户端期望的服务端桥接证书指纹；空值 = 沿用旧行为。
func SetTlsFingerprint(fp string) {
	clientFP = normalizeFingerprint(fp)
}

// GetTlsFingerprint 返回当前客户端期望指纹（空=未固定）。
func GetTlsFingerprint() string {
	return clientFP
}

// TlsDialConfig 构建客户端 tls.Dial 配置（默认不强制）。
func TlsDialConfig() *tls.Config {
	conf := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}
	if clientFP != "" {
		conf.VerifyPeerCertificate = pinVerifier(clientFP)
	}
	return conf
}

func normalizeFingerprint(fp string) string {
	fp = strings.TrimSpace(strings.ToLower(fp))
	fp = strings.TrimPrefix(fp, "sha256:")
	fp = strings.ReplaceAll(fp, ":", "")
	fp = strings.ReplaceAll(fp, " ", "")
	return fp
}

func pinVerifier(fp string) func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return errors.New("tls: no peer certificate")
		}
		sum := sha256.Sum256(rawCerts[0])
		got := hex.EncodeToString(sum[:])
		if got != fp {
			return fmt.Errorf("tls fingerprint mismatch: got %s, want %s", got, fp)
		}
		return nil
	}
}

func generateKeyPair(CommonName string) (rawCert, rawKey []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return
	}
	validFor := time.Hour * 24 * 365 * 10 // ten years
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"My Company Name LTD."},
			CommonName:   CommonName,
			Country:      []string{"US"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return
	}

	rawCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	rawKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return
}
