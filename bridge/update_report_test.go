package bridge

import (
	"testing"

	"ehang.io/nps/lib/common"
)

func TestParseRemoteUpdateReport(t *testing.T) {
	out, ok := ParseRemoteUpdateReport(common.RemoteUpdateReportPrefix + "|ok|cli|done")
	if !ok || out.Status != "ok" || out.Channel != "cli" || out.Message != "done" {
		t.Fatalf("got %+v ok=%v", out, ok)
	}
	if _, ok := ParseRemoteUpdateReport("health-target"); ok {
		t.Fatal("non-update info must not parse")
	}
}
