package controllers

import (
	"os"
	"path/filepath"
	"testing"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
)

func TestNormalizeCatalogPath(t *testing.T) {
	if got := normalizeCatalogPath(`conf\webssl\a.pem`); got != "conf/webssl/a.pem" {
		t.Fatalf("slash normalize: %q", got)
	}
	if got := normalizeCatalogPath("./conf/webssl/a.pem"); got != "conf/webssl/a.pem" {
		t.Fatalf("dot slash: %q", got)
	}
}

func TestPathsReferSame(t *testing.T) {
	if !pathsReferSame("conf/webssl/a.pem", `conf\webssl\a.pem`) {
		t.Fatal("expected same")
	}
	if pathsReferSame("conf/webssl/a.pem", "conf/webssl/b.pem") {
		t.Fatal("expected different")
	}
}

func TestCollectCertificateBindings(t *testing.T) {
	idx := collectCertificateBindings(map[string]string{
		"web_cert_file":    "conf/webssl/panel.pem",
		"web_key_file":     "conf/webssl/panel.key",
		"bridge_cert_file": "conf/bridgessl/b.pem",
	}, []*file.Host{
		{Id: 7, Host: "app.example.com", CertFilePath: "conf/certs/h.pem", KeyFilePath: "conf/certs/h.key"},
	})
	if len(idx[normalizeCatalogPath("conf/webssl/panel.pem")]) == 0 {
		t.Fatal("missing web cert binding")
	}
	if len(idx[normalizeCatalogPath("conf/certs/h.pem")]) == 0 {
		t.Fatal("missing host cert binding")
	}
	labels := bindingsForPaths(idx, "conf/webssl/panel.pem", "conf/webssl/panel.key")
	if len(labels) < 2 {
		t.Fatalf("expected conf bindings, got %v", labels)
	}
}

func TestScanAndDeleteUnusedManagedCertificate(t *testing.T) {
	previous := common.ConfPath
	root := t.TempDir()
	common.ConfPath = root
	t.Cleanup(func() { common.ConfPath = previous })

	webssl := filepath.Join(root, "conf", "webssl")
	if err := os.MkdirAll(webssl, 0700); err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM := writeTestCertificatePair(t, []string{"catalog.example.com"})
	certRel := "conf/webssl/web_unused.pem"
	keyRel := "conf/webssl/web_unused.key"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(certRel)), certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(keyRel)), keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	// 被引用的另一对
	usedCert := "conf/webssl/web_used.pem"
	usedKey := "conf/webssl/web_used.key"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(usedCert)), certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(usedKey)), keyPEM, 0600); err != nil {
		t.Fatal(err)
	}

	bindings := collectCertificateBindings(map[string]string{
		"web_cert_file": usedCert,
		"web_key_file":  usedKey,
	}, nil)
	entries, err := scanManagedCertificateDirectory(webssl, "webssl", bindings)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("expected >=2 entries, got %d", len(entries))
	}
	var unused, used *certificateCatalogEntry
	for i := range entries {
		e := &entries[i]
		switch e.RelCertPath {
		case certRel:
			unused = e
		case usedCert:
			used = e
		}
	}
	if unused == nil || used == nil {
		t.Fatalf("missing entries: unused=%v used=%v all=%+v", unused, used, entries)
	}
	if unused.Referenced || !unused.CanDelete {
		t.Fatalf("unused should be deletable: %+v", unused)
	}
	if !used.Referenced || used.CanDelete {
		t.Fatalf("used must not be deletable: %+v", used)
	}
	if unused.Fingerprint == "" || unused.Subject == "" {
		t.Fatalf("inspection incomplete: %+v", unused)
	}

	if err := deleteUnusedManagedCertificateFiles(usedCert, usedKey, bindings); err == nil {
		t.Fatal("expected refuse delete of referenced pair")
	}
	if err := deleteUnusedManagedCertificateFiles(certRel, keyRel, bindings); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(certRel))); !os.IsNotExist(err) {
		t.Fatalf("cert should be gone: %v", err)
	}
	if err := deleteUnusedManagedCertificateFiles("conf/ssl/evil.pem", "", bindings); err == nil {
		t.Fatal("must refuse non-managed path")
	}
	if err := deleteUnusedManagedCertificateFiles("../etc/passwd", "", bindings); err == nil {
		t.Fatal("must refuse path escape")
	}
}

func TestIsManagedCatalogRelPath(t *testing.T) {
	if !isManagedCatalogRelPath("conf/webssl/a.pem") {
		t.Fatal("webssl")
	}
	if !isManagedCatalogRelPath("conf/bridgessl/a.pem") {
		t.Fatal("bridgessl")
	}
	if !isManagedCatalogRelPath("conf/certs/a.pem") {
		t.Fatal("certs")
	}
	if isManagedCatalogRelPath("conf/ssl/a.pem") {
		t.Fatal("acme ssl must not be managed-deletable")
	}
	if isManagedCatalogRelPath("conf/webssl/../ssl/a.pem") {
		t.Fatal("traversal")
	}
}
