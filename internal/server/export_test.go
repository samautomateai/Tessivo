package server

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/service"
)

func TestExportOutputAPI(t *testing.T) {
	ts, s := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	nb, _ := s.svc.OpenNotebook("ops")
	out, err := s.svc.Run(context.Background(), service.RunRequest{Notebook: nb, Prompt: "Summarize", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := ts.URL + "/v1/notebooks/ops/outputs/" + out.OutputID + "/export?format="
	for format, want := range map[string]string{
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"md":   "text/markdown; charset=utf-8",
	} {
		req, _ := http.NewRequest("GET", base+format, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := ts.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != want || len(b) == 0 ||
			!strings.Contains(resp.Header.Get("Content-Disposition"), "attachment; filename=") {
			t.Fatalf("%s: %d %v", format, resp.StatusCode, resp.Header)
		}
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/ops/outputs/"+out.OutputID+"/diff", ""); code != 400 {
		t.Fatalf("diff of an original: %d", code)
	}
	if code, body := call(t, ts, "GET", "/v1/notebooks/ops/outputs/"+out.OutputID+"/run", ""); code != 200 ||
		!strings.Contains(body, `"prompt":"Summarize"`) {
		t.Fatalf("run info: %d %s", code, body)
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/ops/outputs/"+out.OutputID+"/export?format=rtf", ""); code != 400 {
		t.Fatalf("rtf: %d", code)
	}
}

func TestEditOutputAPI(t *testing.T) {
	ts, s := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"summarize"}`)
	nb, _ := s.svc.OpenNotebook("ops")
	out, err := s.svc.Run(context.Background(), service.RunRequest{Notebook: nb, Prompt: "Summarize", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := "/v1/notebooks/ops/outputs/" + out.OutputID + "/edits"
	edited := `{"data":{"title":"Fixed","tldr":"t","key_points":["k"],"per_source":[],"action_items":[]}}`
	code, body := call(t, ts, "POST", path, edited)
	if code != 201 || !strings.Contains(body, `"parentId":"`+out.OutputID+`"`) ||
		!strings.Contains(body, `"version":2`) {
		t.Fatalf("edit: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", path, `{"data":{"title":"x"}}`); code != 400 {
		t.Fatalf("a result that does not fit the work type: %d", code)
	}
}
