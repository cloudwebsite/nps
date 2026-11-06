package proxy

import (
	"context"
	"testing"

	"ehang.io/nps/lib/file"
	"golang.org/x/crypto/acme/autocert"
)

func TestAutoSSLHostEnabledRequiresManagerAndFlag(t *testing.T) {
	s := &httpServer{httpPort: 80, httpsPort: 443}
	host := &file.Host{AutoSSL: true}
	if s.autoSSLHostEnabled(host) {
		t.Fatal("nil acme manager must disable auto ssl")
	}
	s.acmeManager = &autocert.Manager{}
	if !s.autoSSLHostEnabled(host) {
		t.Fatal("expected auto ssl enabled on 80/443 with flag")
	}
	host.AutoSSL = false
	if s.autoSSLHostEnabled(host) {
		t.Fatal("flag off must disable")
	}
	host.AutoSSL = true
	host.IsClose = true
	if s.autoSSLHostEnabled(host) {
		t.Fatal("closed host must disable")
	}
}

func TestAutoSSLHostEnabledNonStandardPorts(t *testing.T) {
	s := &httpServer{httpPort: 8080, httpsPort: 8443, acmeManager: &autocert.Manager{}}
	host := &file.Host{AutoSSL: true}
	if s.autoSSLHostEnabled(host) {
		t.Fatal("non-standard ports without force must disable")
	}
}

func TestAcmeHostPolicyRejectsIP(t *testing.T) {
	if err := acmeHostPolicy(context.Background(), "1.2.3.4"); err == nil {
		t.Fatal("expected IP rejection")
	}
	if err := acmeHostPolicy(context.Background(), "example.com:443"); err == nil {
		t.Fatal("expected host:port rejection")
	}
}
