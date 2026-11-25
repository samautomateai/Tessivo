package deliver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/testutil"
)

func TestPublishToAFolder(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(config.Integration{Kind: config.IntegrationFolder, Path: dir}, "")
	ref, err := c.Publish(context.Background(), Page{Title: "Q3 ../../report: final", Markdown: "# Q3"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Join(dir, ref.ID)) != dir || !strings.HasSuffix(ref.ID, ".md") {
		t.Fatalf("the file stays in the folder with a safe name: %+v", ref)
	}
	testutil.AssertPerm(t, filepath.Join(dir, ref.ID), 0o600)
	if b, _ := os.ReadFile(filepath.Join(dir, ref.ID)); string(b) != "# Q3" {
		t.Fatalf("content: %q", b)
	}
	again, err := c.Publish(context.Background(), Page{
		Title: "Q3 ../../report: final", Markdown: "# Q3 v2",
		PageID: ref.ID,
	})
	if err != nil || again.ID != ref.ID {
		t.Fatalf("publishing the same output again replaces its file: %v %+v", err, again)
	}
	other, err := c.Publish(context.Background(), Page{Title: "Q3 ../../report: final", Markdown: "other"})
	if err != nil || other.ID == ref.ID {
		t.Fatalf("another output never overwrites a file it did not write: %v %+v", err, other)
	}
	if err := c.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	missing, _ := New(config.Integration{Kind: config.IntegrationFolder, Path: filepath.Join(dir, "nope")}, "")
	if _, err := missing.Publish(context.Background(), Page{Title: "x", Markdown: "x"}); err == nil {
		t.Fatal("a missing folder is an error")
	}
}

func TestPublishToConfluenceCreatesThenUpdates(t *testing.T) {
	var calls []string
	var last map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path)
		last = nil
		_ = json.Unmarshal(b, &last)
		switch r.Method {
		case "POST":
			const page = `{"id":"98","_links":{"base":"https://wiki.test/wiki","webui":"/spaces/ENG/pages/98"}}`
			_, _ = io.WriteString(w, page)
		case "GET":
			_, _ = io.WriteString(w, `{"id":"98","version":{"number":4}}`)
		default:
			_, _ = io.WriteString(w, `{"id":"98","_links":{"base":"https://wiki.test/wiki","webui":"/x/98"}}`)
		}
	}))
	defer ts.Close()
	in := config.Integration{Kind: config.IntegrationConfluence, BaseURL: ts.URL, Space: "ENG", Parent: "12"}
	c, _ := New(in, "t")
	ref, err := c.Publish(context.Background(), Page{Title: "Runbook", Markdown: "# Hi <b>x</b>"})
	if err != nil || ref.ID != "98" || ref.URL != "https://wiki.test/wiki/spaces/ENG/pages/98" {
		t.Fatalf("%v %+v", err, ref)
	}
	raw, _ := json.Marshal(last)
	value, _ := last["body"].(map[string]any)["storage"].(map[string]any)["value"].(string)
	if !strings.Contains(string(raw), `"key":"ENG"`) || !strings.Contains(string(raw), `"id":"12"`) ||
		!strings.Contains(value, "Hi &lt;b&gt;x") || strings.Contains(value, "<b>") {
		t.Fatalf("create body: %s", raw)
	}
	if _, err := c.Publish(context.Background(), Page{Title: "Runbook", Markdown: "v2", PageID: "98"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(calls, ",") != "POST /rest/api/content,GET /rest/api/content/98,PUT /rest/api/content/98" {
		t.Fatalf("calls: %v", calls)
	}
	if v := last["version"].(map[string]any)["number"]; v != float64(5) {
		t.Fatalf("an update bumps the version: %v", v)
	}
}

func TestPublishToNotionSendsBlocksInChunks(t *testing.T) {
	var calls []string
	var sizes []int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Children []any `json:"children"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, r.Method+" "+r.URL.Path)
		sizes = append(sizes, len(body.Children))
		_, _ = io.WriteString(w, `{"id":"pg-1","url":"https://notion.test/pg-1"}`)
	}))
	defer ts.Close()
	c, _ := New(config.Integration{Kind: config.IntegrationNotion, BaseURL: ts.URL, Parent: "parent"}, "t")
	md := strings.Repeat("para\n\n", 250)
	ref, err := c.Publish(context.Background(), Page{Title: "Notes", Markdown: md})
	if err != nil || ref.URL != "https://notion.test/pg-1" {
		t.Fatalf("%v %+v", err, ref)
	}
	if strings.Join(calls, ",") != "POST /v1/pages,PATCH /v1/blocks/pg-1/children,PATCH /v1/blocks/pg-1/children" ||
		sizes[0] != 100 || sizes[1] != 100 || sizes[2] != 50 {
		t.Fatalf("calls %v sizes %v", calls, sizes)
	}
	tracker, _ := New(config.Integration{Kind: config.IntegrationGitHub, Project: "a/b"}, "t")
	if _, err := tracker.Publish(context.Background(), Page{Title: "x", Markdown: "x"}); err == nil {
		t.Fatal("a tracker takes no pages")
	}
}
