package server

import (
	"strings"
	"testing"
)

func TestTemplateAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	code, body := call(t, ts, "GET", "/v1/templates?workType=report", "")
	if code != 200 || !strings.Contains(body, "weekly-ops-report") || strings.Contains(body, "change-risk-review") {
		t.Fatalf("list: %d %s", code, body)
	}
	code, body = call(t, ts, "POST", "/v1/notebooks", `{"templateId":"weekly-ops-report","name":"Ops weekly"}`)
	if code != 201 || !strings.Contains(body, `"template":"weekly-ops-report@1"`) {
		t.Fatalf("create from template: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks", `{"templateId":"nope"}`); code != 404 {
		t.Fatalf("unknown template: %d", code)
	}
	code, body = call(t, ts, "PUT", "/v1/notebooks/ops-weekly/memory", `{"memory":["Week starts Monday"]}`)
	if code != 200 || !strings.Contains(body, "Week starts Monday") {
		t.Fatalf("memory: %d %s", code, body)
	}
	code, body = call(t, ts, "POST", "/v1/templates", `{"fromNotebook":"ops-weekly","name":"Our ops"}`)
	if code != 201 || !strings.Contains(body, `"id":"our-ops"`) {
		t.Fatalf("save as template: %d %s", code, body)
	}
	code, body = call(t, ts, "POST", "/v1/templates", `{"extend":"our-ops","name":"Our ops EU"}`)
	if code != 201 || !strings.Contains(body, `"parent":"our-ops@1"`) {
		t.Fatalf("extend: %d %s", code, body)
	}
	for _, bad := range []string{`{"name":"x"}`, `{"fromNotebook":"a","extend":"b","name":"x"}`} {
		if code, _ := call(t, ts, "POST", "/v1/templates", bad); code != 400 {
			t.Errorf("%s: want 400", bad)
		}
	}
	code, body = call(t, ts, "GET", "/v1/templates/our-ops-eu", "")
	if code != 200 || !strings.Contains(body, "Our ops EU") {
		t.Fatalf("get: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/templates/our-ops-eu", ""); code != 204 {
		t.Fatal("delete")
	}
	if code, _ := call(t, ts, "DELETE", "/v1/templates/change-risk-review", ""); code != 400 {
		t.Fatal("built-ins cannot be deleted")
	}
	if code, _ := call(t, ts, "GET", "/v1/templates/nope", ""); code != 404 {
		t.Fatal("unknown template should 404")
	}
	if code, _ := call(t, ts, "PUT", "/v1/notebooks/nope/memory", `{"memory":[]}`); code != 404 {
		t.Fatal("unknown notebook should 404")
	}
}
