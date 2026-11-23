package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestRenameArchiveDeleteNotebookAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	if code, _ := call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`); code != 201 {
		t.Fatal("create")
	}
	code, body := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"name":"Weekly ops","archived":true}`)
	if code != 200 || !strings.Contains(body, `"name":"Weekly ops"`) || !strings.Contains(body, `"archived":true`) {
		t.Fatalf("patch: %d %s", code, body)
	}
	if code, _ := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"name":""}`); code != 400 {
		t.Fatalf("blank name: %d", code)
	}
	if code, _ := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"bogus":1}`); code != 400 {
		t.Fatalf("unknown field: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/notebooks/ops", ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/notebooks/ops", ""); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
}

func TestAddURLAndTextContextAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	const ctx = "/v1/notebooks/ops/context"
	code, body := call(t, ts, "POST", ctx, `{"url":"https://example.com/status","label":"Status"}`)
	if code != 200 || !strings.Contains(body, `"kind":"url"`) {
		t.Fatalf("url: %d %s", code, body)
	}
	if code, body := call(t, ts, "POST", ctx, `{"text":"Budget 10k","label":"Notes"}`); code != 200 ||
		!strings.Contains(body, `"label":"Notes"`) {
		t.Fatalf("text: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", ctx, `{"url":"ftp://x/y"}`); code != 400 {
		t.Fatalf("bad url: %d", code)
	}
}

func TestChangedElsewhereIsAConflict(t *testing.T) {
	wrapped := fmt.Errorf("rename: %w", notebook.ErrChanged)
	if code, msg := classify(wrapped); code != 409 || !strings.Contains(msg, "changed elsewhere") {
		t.Fatalf("%d %s", code, msg)
	}
}
