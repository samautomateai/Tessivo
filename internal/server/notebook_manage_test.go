package server

import (
	"strings"
	"testing"
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
