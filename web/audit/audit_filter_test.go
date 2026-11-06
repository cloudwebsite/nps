package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListFiltersActorTypeAndSearchesDetails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	Configure(path)
	t.Cleanup(func() { Configure("") })

	_ = Append(Event{Action: "client.connect", ActorType: "bridge", ResourceType: "client", Result: ResultSuccess, Details: map[string]string{"transport": "tls", "version": "0.26.41"}})
	_ = Append(Event{Action: "user.create", ActorType: "admin", ResourceType: "user", Result: ResultSuccess})

	page, err := List(Filter{ActorType: "bridge", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Events[0].Action != "client.connect" {
		t.Fatalf("actor filter failed: %+v", page)
	}

	page, err = List(Filter{Search: "tls", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("detail search failed: %+v", page)
	}
	_ = os.Remove(path)
}
