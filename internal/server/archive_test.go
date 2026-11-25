package server

import (
	"net/http"
	"strings"
	"testing"
)

func zipBody(r *http.Request) { r.Header.Set("Content-Type", "application/zip") }

func TestNotebookArchiveAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	code, zip := call(t, ts, "GET", "/v1/notebooks/ops/archive?outputs=1", "")
	if code != 200 || !strings.HasPrefix(zip, "PK") {
		t.Fatalf("export: %d %.40q", code, zip)
	}
	code, body := call(t, ts, "POST", "/v1/notebooks/import", zip, zipBody)
	if code != 201 || !strings.Contains(body, `"name":"Ops (imported)"`) {
		t.Fatalf("import: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/import", "junk", zipBody); code != 400 {
		t.Fatalf("junk import: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/import", zip); code != 400 {
		t.Fatalf("import without a zip content type: %d", code)
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/nope/archive", ""); code != 404 {
		t.Fatalf("missing notebook: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/ops/context/x/relink", `{"path":"rel"}`); code != 400 {
		t.Fatalf("relink a relative path: %d", code)
	}
}

func TestResearchArchiveAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/research", `{"name":"Vendors"}`)
	code, zip := call(t, ts, "GET", "/v1/research/vendors/archive", "")
	if code != 200 {
		t.Fatalf("export: %d", code)
	}
	code, body := call(t, ts, "POST", "/v1/research/import", zip, zipBody)
	if code != 201 || !strings.Contains(body, `"name":"Vendors (imported)"`) {
		t.Fatalf("import: %d %s", code, body)
	}
}
