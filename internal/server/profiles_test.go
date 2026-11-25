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
