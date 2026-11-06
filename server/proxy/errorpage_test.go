package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ehang.io/nps/lib/file"
)

func setErrorPageBrandForTest(t *testing.T, brand, brandURL string) {
	t.Helper()
	db := file.GetDb()
	g := db.GetGlobal()
	if g == nil {
		g = new(file.Glob)
	}
	b, u := brand, brandURL
	g.HttpErrorPageBrand = &b
	g.HttpErrorPageBrandURL = &u
	if err := db.SaveGlobal(g); err != nil {
		t.Fatal(err)
	}
}

func TestRenderErrorPagePlaceholders(t *testing.T) {
	setErrorPageBrandForTest(t, "AcmeNPS", "https://example.com")

	tpl := []byte(`{{status}}|{{host}}|{{message}}|{{time}}|{{brand}}|{{brand_url}}|{{brand_link}}`)
	out := string(RenderErrorPage(tpl, "502", `<script>x</script>`, `a&b`))
	if !strings.Contains(out, "502|") {
		t.Fatalf("status: %s", out)
	}
	if strings.Contains(out, "<script>") {
		t.Fatalf("host not escaped: %s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("expected escaped host: %s", out)
	}
	if !strings.Contains(out, "a&amp;b") {
		t.Fatalf("message escape: %s", out)
	}
	if !strings.Contains(out, "AcmeNPS") || !strings.Contains(out, `href="https://example.com"`) {
		t.Fatalf("brand link: %s", out)
	}
	if strings.Contains(out, "{{") {
		t.Fatalf("leftover placeholders: %s", out)
	}
}

func TestRenderErrorPageUnknownPlaceholderKept(t *testing.T) {
	out := string(RenderErrorPage([]byte(`{{status}} {{unknown}}`), "404", "h", "m"))
	if !strings.Contains(out, "{{unknown}}") {
		t.Fatalf("unknown should remain: %s", out)
	}
}

func TestLoadErrorPageCustomPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.html")
	if err := os.WriteFile(custom, []byte("CUSTOM {{status}} {{brand}}"), 0644); err != nil {
		t.Fatal(err)
	}
	setErrorPageBrandForTest(t, "nps", "")
	errorPageMu.Lock()
	errorPageTemplate = []byte("CUSTOM {{status}} {{brand}}")
	errorPageMu.Unlock()
	out := string(RenderErrorPage(nil, "502", "h", "m"))
	if !strings.HasPrefix(out, "CUSTOM 502") {
		t.Fatalf("got %q", out)
	}
	if strings.Contains(out, "<a href") {
		t.Fatalf("empty brand_url should not link: %s", out)
	}
}
