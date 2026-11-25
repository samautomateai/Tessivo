package server

import (
	"strings"
	"testing"
)

func TestTagsAndPinsAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Board","workType":"summarize"}`)
	code, body := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"tags":["Infra"],"pinned":true}`)
	if code != 200 || !strings.Contains(body, `"tags":["Infra"]`) || !strings.Contains(body, `"pinned":true`) {
		t.Fatalf("%d %s", code, body)
	}
	code, body = call(t, ts, "GET", "/v1/notebooks?tag=infra", "")
	if code != 200 || !strings.Contains(body, `"ops"`) || strings.Contains(body, `"board"`) {
		t.Fatalf("filter: %d %s", code, body)
	}
	if code, _ := call(t, ts, "PATCH", "/v1/notebooks/ops", `{"tags":["a/b"]}`); code != 400 {
		t.Fatalf("bad tag: %d", code)
	}
	call(t, ts, "POST", "/v1/research", `{"name":"Vendors"}`)
	code, body = call(t, ts, "PATCH", "/v1/research/vendors", `{"tags":["q3"],"pinned":true}`)
	if code != 200 || !strings.Contains(body, `"tags":["q3"]`) || !strings.Contains(body, `"pinned":true`) {
		t.Fatalf("research: %d %s", code, body)
	}
}
