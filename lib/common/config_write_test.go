package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceIniKeyValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nps.conf")
	if err := os.WriteFile(path, []byte("web_password=plain\nweb_username=admin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceIniKeyValue(path, "web_password", "$2a$10$hash"); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "web_password=$2a$10$hash") {
		t.Fatalf("%s", body)
	}
	if !strings.Contains(string(body), "web_username=admin") {
		t.Fatal("other keys must stay")
	}
}
