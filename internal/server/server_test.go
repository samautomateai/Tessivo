package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

const token = "test-token-0123456789"

type fakeModel struct{ reply string }

func (f fakeModel) ChatStream(_ context.Context, _ llm.Request, onDelta func(string)) (llm.Result, error) {
	onDelta(f.reply)
	return llm.Result{Response: llm.Response{Content: f.reply}, Target: "litellm/m"}, nil
}

const summary = `{"title":"Digest","tldr":"t","key_points":["k"],"per_source":[],"action_items":[]}`

func newTestServer(t *testing.T, reply string) (*httptest.Server, *Server) {
	t.Helper()
	f := config.Default()
	f.Defaults.Model = "litellm/m"
	dir := t.TempDir()
	svc := &service.Service{
		Config: f, Secrets: secrets.Map{}, Workspace: filepath.Join(dir, "ws"),
		UserAuditPath: filepath.Join(dir, "audit.jsonl"),
		NewModel: func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
			return fakeModel{reply}, nil
		},
	}
	s := New(svc, token, "test")
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func call(t *testing.T, ts *httptest.Server, method, path, body string, mutate ...func(*http.Request)) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, m := range mutate {
		m(req)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestSecurityGates(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	noAuth := func(r *http.Request) { r.Header.Del("Authorization") }
	if code, body := call(t, ts, "GET", "/v1/health", "", noAuth); code != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("health: %d %s", code, body)
	}
	cases := map[string]func(*http.Request){
		"no token":    func(r *http.Request) { r.Header.Del("Authorization") },
		"bad token":   func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") },
		"browser":     func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"fetch meta":  func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"rebind host": func(r *http.Request) { r.Host = "evil.example:80" },
	}
	want := map[string]int{"no token": 401, "bad token": 401, "browser": 403, "fetch meta": 403, "rebind host": 421}
	for name, m := range cases {
		if code, _ := call(t, ts, "GET", "/v1/notebooks", "", m); code != want[name] {
			t.Errorf("%s: got %d want %d", name, code, want[name])
		}
	}
	code, _ := call(t, ts, "GET", "/v1/notebooks", "")
	if code != 200 {
		t.Fatalf("authorised request: %d", code)
	}
	for _, h := range []string{"", "tessivo", "localhost", "localhost:9", "127.0.0.1:5", "[::1]:5"} {
		if !allowedHost(h) {
			t.Errorf("host %q should be allowed", h)
		}
	}
}

func plainText(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }

func TestRequestValidation(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	cases := []struct {
		body   string
		mutate func(*http.Request)
		want   int
	}{
		{`{"name":"x","workType":"summarize","extra":1}`, nil, 400},
		{`{"name":"x"} {"again":1}`, nil, 400},
		{`not json`, nil, 400},
		{`{"name":"x","workType":"summarize"}`, plainText, 400},
		{`{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`, nil, 400},
		{`{"name":"x","workType":"poetry"}`, nil, 400},
	}
	for i, c := range cases {
		var ms []func(*http.Request)
		if c.mutate != nil {
			ms = append(ms, c.mutate)
		}
		if code, body := call(t, ts, "POST", "/v1/notebooks", c.body, ms...); code != c.want {
			t.Errorf("case %d: got %d want %d (%s)", i, code, c.want, body)
		}
	}
}

func TestNotebookAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	code, body := call(t, ts, "POST", "/v1/notebooks", `{"name":"Board Minutes","workType":"summarize"}`)
	if code != 201 || !strings.Contains(body, `"id":"board-minutes"`) {
		t.Fatalf("create: %d %s", code, body)
	}
	src := filepath.Join(t.TempDir(), "notes.md")
	_ = os.WriteFile(src, []byte("Ship in March"), 0o600)
	pathJSON, _ := json.Marshal(map[string]string{"path": src})
	code, body = call(t, ts, "POST", "/v1/notebooks/board-minutes/context", string(pathJSON))
	if code != 200 || !strings.Contains(body, `"estimatedTokens"`) {
		t.Fatalf("add context: %d %s", code, body)
	}
	var nb service.NotebookInfo
	_ = json.Unmarshal([]byte(body), &nb)
	ctxPath := "/v1/notebooks/board-minutes/context/" + nb.Context[0].ID
	if code, _ := call(t, ts, "PATCH", ctxPath, `{"enabled":false}`); code != 200 {
		t.Fatalf("toggle: %d", code)
	}
	if code, _ := call(t, ts, "PATCH", ctxPath, `{}`); code != 400 {
		t.Fatalf("toggle without value: %d", code)
	}
	if code, _ := call(t, ts, "PATCH", "/v1/notebooks/board-minutes/context/nope", `{"enabled":true}`); code != 404 {
		t.Fatal("unknown context should 404")
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/board-minutes/context", `{"path":"rel.md"}`); code != 400 {
		t.Fatal("relative path should 400")
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/..%2Fetc", ""); code != 400 && code != 404 {
		t.Fatalf("traversal: %d", code)
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/missing", ""); code != 404 {
		t.Fatal("missing notebook should 404")
	}
	if code, body := call(t, ts, "GET", "/v1/notebooks/board-minutes", ""); code != 200 ||
		!strings.Contains(body, "Board Minutes") {
		t.Fatalf("get: %d %s", code, body)
	}
	if code, body := call(t, ts, "GET", "/v1/worktypes", ""); code != 200 || !strings.Contains(body, "analyze") {
		t.Fatalf("worktypes: %d", code)
	}
	if code, body := call(t, ts, "GET", "/v1/settings", ""); code != 200 || !strings.Contains(body, "api_key_ref") {
		t.Fatalf("settings: %d %s", code, body)
	}
	if code, _ := call(t, ts, "GET", "/v1/models", ""); code != 500 {
		t.Fatalf("models with unreachable provider should 500, got %d", code)
	}
}

type sseEvent struct{ name, data string }

func readEvents(t *testing.T, r io.Reader) []sseEvent {
	t.Helper()
	var out []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "" && cur.name != "":
			out = append(out, cur)
			cur = sseEvent{}
		}
	}
	return out
}

func runRequest(t *testing.T, ts *httptest.Server, id, body string) []sseEvent {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/v1/notebooks/"+id+"/runs", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("content type %q: %s", ct, b)
	}
	return readEvents(t, resp.Body)
}

func TestRunStreamsEvents(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Digest","workType":"summarize"}`)
	evs := runRequest(t, ts, "digest", `{"prompt":"Summarize","overrides":{"thinking_effort":"low"}}`)
	names := make([]string, 0, len(evs))
	for _, e := range evs {
		names = append(names, e.name)
	}
	if got := strings.Join(names, ","); got != "status,delta,finished" {
		t.Fatalf("events: %s", got)
	}
	var fin map[string]any
	_ = json.Unmarshal([]byte(evs[2].data), &fin)
	if fin["valid"] != true || fin["outputId"] == "" || !strings.Contains(fin["markdown"].(string), "# Digest") {
		t.Fatalf("finished: %v", fin)
	}
	code, body := call(t, ts, "GET", "/v1/outputs?workType=summarize", "")
	if code != 200 || !strings.Contains(body, `"notebookId":"digest"`) {
		t.Fatalf("outputs: %d %s", code, body)
	}
	path := "/v1/notebooks/digest/outputs/" + fin["outputId"].(string)
	if code, body := call(t, ts, "GET", path, ""); code != 200 || !strings.Contains(body, "Digest") {
		t.Fatalf("output: %d", code)
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/digest/outputs/nope", ""); code != 404 {
		t.Fatal("missing output should 404")
	}
}

func TestRunErrorsAndLimits(t *testing.T) {
	ts, s := newTestServer(t, "not json")
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Bad","workType":"summarize"}`)
	evs := runRequest(t, ts, "bad", `{}`)
	last := evs[len(evs)-1]
	if last.name != "error" || !strings.Contains(last.data, `"status":422`) {
		t.Fatalf("want 422 error event, got %+v", last)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/bad/runs", `{"overrides":{"mode":"warp"}}`); code != 200 {
		t.Fatalf("invalid overrides stream an error event, got %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/ghost/runs", `{}`); code != 404 {
		t.Fatal("unknown notebook should 404")
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/bad/runs", `{"x":1}`); code != 400 {
		t.Fatal("unknown field should 400")
	}
	for i := 0; i < MaxConcurrentRuns; i++ {
		s.runs <- struct{}{}
	}
	if code, _ := call(t, ts, "POST", "/v1/notebooks/bad/runs", `{}`); code != 429 {
		t.Fatalf("want 429 when saturated, got %d", code)
	}
}

func TestPanicRecovery(t *testing.T) {
	_, s := newTestServer(t, summary)
	h := s.secure(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://localhost/v1/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(rec, req)
	if rec.Code != 500 || strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("panic: %d %s", rec.Code, rec.Body)
	}
}

func TestUnixSocketEndToEnd(t *testing.T) {
	dir, err := os.MkdirTemp("", "tsv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "run", "core.sock")
	ln, err := Listen(context.Background(), "unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(sock); info.Mode().Perm() != 0o600 {
		t.Fatalf("socket perm %v", info.Mode().Perm())
	}
	_, err = Listen(context.Background(), "unix", sock)
	if err == nil || !strings.Contains(err.Error(), "already listening") {
		t.Fatalf("want already listening, got %v", err)
	}
	_, s := newTestServer(t, summary)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, s.Handler()) }()

	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", sock)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: dial}}
	req, _ := http.NewRequest("GET", "http://tessivo/v1/notebooks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("unix request: %v %v", err, resp)
	}
	_ = resp.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
	// Stale socket is replaced; a regular file is refused.
	if ln2, err := Listen(context.Background(), "unix", sock); err != nil {
		t.Fatalf("stale socket: %v", err)
	} else {
		_ = ln2.Close()
	}
	file := filepath.Join(dir, "file")
	_ = os.WriteFile(file, nil, 0o600)
	if _, err := Listen(context.Background(), "unix", file); err == nil {
		t.Fatal("want not-a-socket error")
	}
}

func TestListenTCPAndEndpoint(t *testing.T) {
	for _, bad := range []string{"0.0.0.0:0", "192.168.1.5:0", "nohost", ":0"} {
		if _, err := Listen(context.Background(), "tcp", bad); err == nil {
			t.Errorf("%s: want refusal", bad)
		}
	}
	if _, err := Listen(context.Background(), "udp", "x"); err == nil {
		t.Fatal("want unknown network")
	}
	ln, err := Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	_ = ln.Close()
	if err := Serve(ctx, ln, http.NotFoundHandler()); err == nil {
		t.Fatal("want serve error on closed listener")
	}
	cancel()
	p := filepath.Join(t.TempDir(), "run", "endpoint.json")
	tok, err := NewToken()
	if err != nil || len(tok) != 64 {
		t.Fatalf("token %q %v", tok, err)
	}
	if err := WriteEndpoint(p, Endpoint{Network: "tcp", Address: "127.0.0.1:1", Token: tok}); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
		t.Fatalf("endpoint perm %v", info.Mode().Perm())
	}
	if err := WriteEndpoint("/proc/tessivo/endpoint.json", Endpoint{}); err == nil {
		t.Fatal("want write error")
	}
}
