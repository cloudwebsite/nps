package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConfigFileAddsMissingRequiredKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nps.conf")
	// 模拟较早生产：缺 tls 相关键
	old := "bridge_type=tcp\nbridge_port=8024\nbridge_ip=0.0.0.0\nweb_username=admin\nweb_password=secret\n"
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	changed, added, err := migrateConfigFile(path, defaultNpsConf)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected config to change")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "web_password=secret") {
		t.Fatal("must preserve existing password")
	}
	if !strings.Contains(text, "tls_enable=") {
		t.Fatalf("expected tls_enable to be added, got added=%v body=%s", added, text)
	}
	// 幂等
	changed2, _, err := migrateConfigFile(path, defaultNpsConf)
	if err != nil {
		t.Fatal(err)
	}
	if changed2 {
		t.Fatal("second migrate must be idempotent")
	}
}

func TestMigrateConfigCompatibilityBridgeTLSAlias(t *testing.T) {
	content := "bridge_type=tcp\nbridge_tls_port=8025\n"
	out, added := migrateConfigCompatibility(content)
	if len(added) != 1 || added[0] != "tls_bridge_port" {
		t.Fatalf("added=%v", added)
	}
	if !strings.Contains(out, "tls_bridge_port=8025") {
		t.Fatalf("out=%s", out)
	}
	if !strings.Contains(out, "bridge_tls_port=8025") {
		t.Fatal("old alias must remain for rollback")
	}
}
