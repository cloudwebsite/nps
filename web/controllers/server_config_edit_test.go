package controllers

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteServerConfigUpdatesPreservesCommentsAndUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nps.conf")
	original := "# keep me\n" +
		"bridge_port=8024\n" +
		"custom_unknown=keep\n" +
		"web_open_ssl=false\n" +
		"log_level=6\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := writeServerConfigUpdates(path, []confUpdate{
		{Key: "bridge_port", Value: "9000"},
		{Key: "audit_log_path", Value: "conf/audit.log"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("changed=%v", changed)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "# keep me") {
		t.Fatalf("comment lost: %s", text)
	}
	if !strings.Contains(text, "custom_unknown=keep") {
		t.Fatalf("unknown key lost: %s", text)
	}
	if !strings.Contains(text, "bridge_port=9000") {
		t.Fatalf("bridge_port not updated: %s", text)
	}
	if !strings.Contains(text, "audit_log_path=conf/audit.log") {
		t.Fatalf("new key missing: %s", text)
	}
	backs, _ := filepath.Glob(path + ".bak.*")
	if len(backs) == 0 {
		t.Fatal("expected backup file")
	}
}

func TestWriteServerConfigUpdatesOmitOptional(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nps.conf")
	if err := os.WriteFile(path, []byte("bridge_port=8024\naudit_log_path=conf/audit.log\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := writeServerConfigUpdates(path, []confUpdate{
		{Key: "audit_log_path", Omit: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "audit_log_path" {
		t.Fatalf("changed=%v", changed)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "audit_log_path=") {
		t.Fatalf("omit failed: %s", got)
	}
}

func TestParseConfUpdatesKeepsSensitiveEmpty(t *testing.T) {
	content := "" +
		"http_proxy_ip=0.0.0.0\n" +
		"bridge_type=tcp\n" +
		"bridge_port=8024\n" +
		"bridge_ip=0.0.0.0\n" +
		"web_username=admin\n" +
		"web_password=secret\n" +
		"web_port=8081\n" +
		"web_ip=0.0.0.0\n" +
		"web_open_ssl=false\n" +
		"auth_key=abc\n" +
		"auth_crypt_key=1234567890123456\n" +
		"log_level=6\n"

	form := url.Values{}
	for _, meta := range confEditableCatalog {
		cur, present := confKeyValues(content)[meta.Key]
		if meta.Optional {
			if present {
				form.Set("config_enabled_"+meta.Key, "1")
				form.Set("config_"+meta.Key, cur)
			}
			continue
		}
		if meta.Sensitive {
			form.Set("config_"+meta.Key, "")
			continue
		}
		form.Set("config_"+meta.Key, cur)
	}
	form.Set("config_bridge_port", "8025")

	req, err := http.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	updates, err := parseConfUpdates(req, content)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range updates {
		if u.Key == "web_password" || u.Key == "auth_key" || u.Key == "auth_crypt_key" {
			t.Fatalf("sensitive empty should not update: %+v", u)
		}
	}
	foundPort := false
	for _, u := range updates {
		if u.Key == "bridge_port" && u.Value == "8025" {
			foundPort = true
		}
	}
	if !foundPort {
		t.Fatalf("bridge_port update missing: %+v", updates)
	}
}

func TestValidateConfUpdateRelationsWebTLS(t *testing.T) {
	err := validateConfUpdateRelations(map[string]string{}, []confUpdate{
		{Key: "web_open_ssl", Value: "true"},
	})
	if err == nil {
		t.Fatal("expected cert pair error")
	}
}

func TestMaskConfContent(t *testing.T) {
	got := maskConfContent("web_password=123\nbridge_port=1\n")
	if !strings.Contains(got, "web_password=******") {
		t.Fatalf("mask failed: %s", got)
	}
	if !strings.Contains(got, "bridge_port=1") {
		t.Fatalf("non-sensitive altered: %s", got)
	}
}
