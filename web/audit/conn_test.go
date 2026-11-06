package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordConnectionRespectsSwitch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	Configure(path)
	SetConnectionEnabled(false)
	SetConnectionMinInterval(0)
	RecordConnection("client.connect", 1, 0, "1.2.3.4", nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disabled connection audit should not create journal")
	}
	SetConnectionEnabled(true)
	RecordConnection("client.connect", 2, 9, "5.6.7.8", nil)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "client.connect") || !strings.Contains(string(data), `"resource_id":2`) {
		t.Fatalf("unexpected journal: %s", data)
	}
	Configure("")
	SetConnectionEnabled(false)
}

func TestRecordConnectionDebounce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	Configure(path)
	SetConnectionEnabled(true)
	SetConnectionMinInterval(60)
	RecordConnection("client.connect", 7, 0, "1.1.1.1", nil)
	RecordConnection("client.connect", 7, 0, "1.1.1.1", nil)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("expected 1 debounced line, got %q", data)
	}
	Configure("")
	SetConnectionEnabled(false)
	SetConnectionMinInterval(0)
}
