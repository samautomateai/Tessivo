package server

import (
	"strings"
	"testing"
)

func TestPromptAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	code, body := call(t, ts, "PUT", "/v1/notebooks/ops/prompts/weekly",
		`{"text":"Summarize {{week}}","settings":{"temperature":0.2}}`)
	if code != 200 || !strings.Contains(body, `"name":"weekly"`) {
		t.Fatalf("put: %d %s", code, body)
	}
	if code, body := call(t, ts, "GET", "/v1/notebooks/ops/prompts", ""); code != 200 ||
		!strings.Contains(body, `"temperature":0.2`) {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, _ := call(t, ts, "PUT", "/v1/notebooks/ops/prompts/weekly", `{"text":""}`); code != 400 {
		t.Fatalf("empty text: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/notebooks/ops/prompts/weekly", ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/notebooks/ops/prompts/weekly", ""); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
}

func TestSearchAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Payments risk","workType":"summarize"}`)
	if code, body := call(t, ts, "GET", "/v1/search?q=payments", ""); code != 200 ||
		!strings.Contains(body, `"kind":"notebook"`) {
		t.Fatalf("search: %d %s", code, body)
	}
	if code, _ := call(t, ts, "GET", "/v1/search?q=", ""); code != 400 {
		t.Fatalf("blank: %d", code)
	}
}
