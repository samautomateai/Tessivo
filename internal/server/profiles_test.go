package server

import (
	"strings"
	"testing"
)

func TestProfilesAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	if code, body := call(t, ts, "POST", "/v1/profiles", `{"name":"work"}`); code != 200 ||
		!strings.Contains(body, `"active":"work"`) {
		t.Fatalf("save: %d %s", code, body)
	}
	call(t, ts, "POST", "/v1/profiles", `{"name":"home"}`)
	if code, body := call(t, ts, "POST", "/v1/profiles/work/use", ""); code != 200 ||
		!strings.Contains(body, `"active":"work"`) {
		t.Fatalf("use: %d %s", code, body)
	}
	if code, body := call(t, ts, "GET", "/v1/profiles", ""); code != 200 || strings.Count(body, `"name"`) != 2 {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/profiles/nope/use", ""); code != 404 {
		t.Fatalf("unknown: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/profiles", `{"name":"Bad Name"}`); code != 400 {
		t.Fatalf("bad name: %d", code)
	}
	code, body := call(t, ts, "DELETE", "/v1/profiles/work", "")
	if code != 200 || !strings.Contains(body, `"active":""`) {
		t.Fatalf("delete: %d %s", code, body)
	}
}

func TestWorkTypeSuggestionAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	if code, body := call(t, ts, "GET", "/v1/notebooks/ops/worktype-suggestion", ""); code != 200 || body != "{}\n" {
		t.Fatalf("no sources, no suggestion: %d %q", code, body)
	}
	if code, body := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"workType":"report"}`); code != 200 ||
		!strings.Contains(body, `"workType":"report"`) {
		t.Fatalf("switch: %d %s", code, body)
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/nope/worktype-suggestion", ""); code != 404 {
		t.Fatalf("unknown notebook: %d", code)
	}
}
