package notebook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddURLAndText(t *testing.T) {
	nb, err := Create(filepath.Join(t.TempDir(), "nb"), "", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	u, err := nb.AddURL("https://status.example.com/q3", "")
	if err != nil || !u.IsURL() || u.Label != "status.example.com/q3" || u.URI != "https://status.example.com/q3" {
		t.Fatalf("%v %+v", err, u)
	}
	for _, bad := range []string{"ftp://x/y", "http://u:p@x/", "", "nope"} {
		if _, err := nb.AddURL(bad, ""); err == nil {
			t.Errorf("added %q", bad)
		}
	}
	item, err := nb.AddText("Meeting notes", "Decided: ship in March.")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(nb.ContextPath(item))
	if err != nil || string(b) != "Decided: ship in March.\n" || !strings.HasSuffix(item.Path, ".md") {
		t.Fatalf("%v %q %+v", err, b, item)
	}
	for _, bad := range [][2]string{{"x", ""}, {"x", "  "}, {"x", strings.Repeat("a", MaxTextSource+1)}} {
		if _, err := nb.AddText(bad[0], bad[1]); err == nil {
			t.Errorf("added text of %d bytes", len(bad[1]))
		}
	}
	reopened, _ := Open(nb.Dir)
	if len(reopened.Manifest.Context) != 2 {
		t.Fatalf("context: %+v", reopened.Manifest.Context)
	}
}

func TestURLItemsHaveNoPathOnDisk(t *testing.T) {
	nb, _ := Create(filepath.Join(t.TempDir(), "nb"), "", "analyze")
	u, err := nb.AddURL("https://example.com/a", "")
	if err != nil {
		t.Fatal(err)
	}
	if p := nb.ContextPath(u); p != "" {
		t.Fatalf("a web page resolved to %q (the notebook folder would become an agent root)", p)
	}
}
