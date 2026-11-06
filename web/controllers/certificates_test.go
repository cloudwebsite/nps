package controllers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ehang.io/nps/lib/common"
)

func writeTestCertificatePair(t *testing.T, dnsNames []string) (certPEM, keyPEM []byte) {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	subject := "cert.test"
	if len(dnsNames) > 0 && strings.TrimSpace(dnsNames[0]) != "" {
		subject = dnsNames[0]
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: subject},
		DNSNames:              append([]string(nil), dnsNames...),
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func TestInspectCertificatePairValidAndHostname(t *testing.T) {
	certPEM, keyPEM := writeTestCertificatePair(t, []string{"app.example.com"})
	inspection, err := inspectCertificatePair(certPEM, keyPEM, "app.example.com", false)
	if err != nil {
		t.Fatalf("valid pair rejected: %v", err)
	}
	if !inspection.Valid || !inspection.HostnameOK || inspection.Fingerprint == "" {
		t.Fatalf("inspection incomplete: %+v", inspection)
	}
	if _, err := inspectCertificatePair(certPEM, keyPEM, "other.example.com", false); err == nil {
		t.Fatal("expected hostname mismatch")
	}
}

func TestPersistCertificatePairWritesManagedPaths(t *testing.T) {
	previous := common.ConfPath
	common.ConfPath = t.TempDir()
	t.Cleanup(func() { common.ConfPath = previous })

	certPEM, keyPEM := writeTestCertificatePair(t, []string{"bridge.example.com"})
	certPath, keyPath, err := persistCertificatePair("bridge", "server", certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	if certPath != "conf/bridgessl/bridge_server.pem" || keyPath != "conf/bridgessl/bridge_server.key" {
		t.Fatalf("paths = %q, %q", certPath, keyPath)
	}
	for _, rel := range []string{certPath, keyPath} {
		if _, err := os.Stat(filepath.Join(common.ConfPath, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing managed file %s: %v", rel, err)
		}
	}
}

func TestCertificateMaterialFromRequestPasteAndPath(t *testing.T) {
	previous := common.ConfPath
	common.ConfPath = t.TempDir()
	t.Cleanup(func() { common.ConfPath = previous })
	if err := os.MkdirAll(filepath.Join(common.ConfPath, "conf"), 0755); err != nil {
		t.Fatal(err)
	}

	certPEM, keyPEM := writeTestCertificatePair(t, []string{"panel.example.com"})
	form := url.Values{
		"cert": {base64.StdEncoding.EncodeToString(certPEM)},
		"key":  {base64.StdEncoding.EncodeToString(keyPEM)},
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	gotCert, gotKey, err := certificateMaterialFromRequest(req, "paste")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCert) != string(certPEM) || string(gotKey) != string(keyPEM) {
		t.Fatal("paste material mismatch")
	}

	certFile := filepath.Join(common.ConfPath, "conf", "a.pem")
	keyFile := filepath.Join(common.ConfPath, "conf", "a.key")
	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	pathForm := url.Values{"cert": {"conf/a.pem"}, "key": {"conf/a.key"}}
	pathReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(pathForm.Encode()))
	pathReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	gotCert, gotKey, err = certificateMaterialFromRequest(pathReq, "path")
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCert) != string(certPEM) || string(gotKey) != string(keyPEM) {
		t.Fatal("path material mismatch")
	}
}

func TestValidateConfUpdateRelationsCertificatePairs(t *testing.T) {
	err := validateConfUpdateRelations(map[string]string{}, []confUpdate{
		{Key: "bridge_cert_file", Value: "conf/a.pem"},
	})
	if err == nil {
		t.Fatal("expected bridge pair error")
	}
	if err := validateConfUpdateRelations(map[string]string{
		"web_open_ssl":  "true",
		"web_cert_file": "conf/a.pem",
		"web_key_file":  "conf/a.key",
	}, nil); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
