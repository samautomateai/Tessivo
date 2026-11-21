package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

// agentModel replies from a queue; gate, when set, is waited on before each reply.
type agentModel struct {
	mu      sync.Mutex
	replies []llm.Response
	gate    chan struct{}
}

func (m *agentModel) ChatStream(ctx context.Context, _ llm.Request, _ func(string)) (llm.Result, error) {
	if m.gate != nil {
		select {
		case <-m.gate:
		case <-ctx.Done():
			return llm.Result{}, ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.replies) == 0 {
		return llm.Result{}, errors.New("no reply queued")
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return llm.Result{Response: r, Target: "litellm/m"}, nil
}

func grepCall() llm.Response {
	return llm.Response{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "grep", Arguments: `{"pattern":"x"}`}}}
}

type agentServer struct {
	ts    *httptest.Server
	svc   *service.Service
	model *agentModel
}

func newAgentServer(t *testing.T, replies ...llm.Response) *agentServer {
	t.Helper()
	m := &agentModel{replies: replies}
	f := config.Default()
	f.Defaults.Model = "litellm/m"
	f.Agent.Tools = map[string]string{"grep": "ask"}
	dir := t.TempDir()
	svc := &service.Service{
		Config: f, Secrets: secrets.Map{}, Workspace: filepath.Join(dir, "ws"),
		UserAuditPath: filepath.Join(dir, "audit.jsonl"),
		NewModel:      func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil },
	}
	s := New(svc, token, "test")
	s.Events = NewBroker()
	svc.OnRunEvent = func(e service.RunEvent) { s.Events.Publish(e) }
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Repo","workType":"summarize"}`)
	body, _ := json.Marshal(map[string]string{"path": src})
	if code, out := call(t, ts, "POST", "/v1/notebooks/repo/context", string(body)); code != 200 {
		t.Fatalf("add context: %d %s", code, out)
	}
	return &agentServer{ts: ts, svc: svc, model: m}
}

const agentBody = `{"overrides":{"mode":"agent"}}`

func pausedRun(t *testing.T, a *agentServer) (runID, approvalID string) {
	t.Helper()
	evs := runRequest(t, a.ts, "repo", agentBody)
	last := evs[len(evs)-1]
	if last.name != "paused" {
		t.Fatalf("want a paused event last, got %+v", evs)
	}
	var p struct {
		RunID     string                    `json:"runId"`
		Approvals []service.PendingApproval `json:"approvals"`
	}
	if err := json.Unmarshal([]byte(last.data), &p); err != nil || len(p.Approvals) != 1 {
		t.Fatalf("paused payload: %s %v", last.data, err)
	}
	hasToolCall := false
	for _, e := range evs {
		hasToolCall = hasToolCall || e.name == "approval_needed"
	}
	if !hasToolCall {
		t.Fatalf("the stream must carry approval_needed: %+v", evs)
	}
	return p.RunID, p.Approvals[0].ApprovalID
}

func waitStatus(t *testing.T, a *agentServer, runID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nb, err := a.svc.OpenNotebook("repo")
		if err == nil {
			if r, err := nb.LoadRun(runID); err == nil && r.Status == want {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never reached %s", runID, want)
}

func TestApprovalFlowHTTP(t *testing.T) {
	a := newAgentServer(t, grepCall(), llm.Response{Content: "done"}, llm.Response{Content: summary})
	runID, aid := pausedRun(t, a)
	code, body := call(t, a.ts, "GET", "/v1/approvals", "")
	if code != 200 || !strings.Contains(body, aid) {
		t.Fatalf("approvals: %d %s", code, body)
	}
	path := fmt.Sprintf("/v1/runs/%s/approvals/%s", runID, aid)
	code, body = call(t, a.ts, "POST", path, `{"decision":"allow"}`)
	if code != 202 || !strings.Contains(body, "resuming") {
		t.Fatalf("answer: %d %s", code, body)
	}
	waitStatus(t, a, runID, notebook.StatusFinished)
	if code, body := call(t, a.ts, "GET", "/v1/runs/"+runID+"/events", ""); code != 200 ||
		!strings.Contains(body, notebook.StatusFinished) {
		t.Fatalf("events of a finished run: %d %s", code, body)
	}
}

func TestApprovalHTTPErrors(t *testing.T) {
	a := newAgentServer(t, grepCall())
	runID, aid := pausedRun(t, a)
	unknown := "/v1/runs/20990101T000000-000000/approvals/x"
	if code, _ := call(t, a.ts, "POST", unknown, `{"decision":"allow"}`); code != 404 {
		t.Fatalf("unknown run: %d", code)
	}
	path := fmt.Sprintf("/v1/runs/%s/approvals/%s", runID, aid)
	if code, _ := call(t, a.ts, "POST", path, `{"decision":"maybe"}`); code != 400 {
		t.Fatalf("bad decision: %d", code)
	}
	if code, _ := call(t, a.ts, "POST", "/v1/runs/"+runID+"/resume", ""); code != 409 {
		t.Fatalf("resume before answering: %d", code)
	}
	if code, _ := call(t, a.ts, "POST", path, `{"decision":"deny"}`); code != 202 {
		t.Fatalf("deny: %d", code)
	}
	if code, _ := call(t, a.ts, "POST", path, `{"decision":"deny"}`); code != 409 && code != 404 {
		t.Fatalf("second answer: %d", code)
	}
}

func TestDisconnectDoesNotCancelAgentRun(t *testing.T) {
	a := newAgentServer(t, llm.Response{Content: "done"}, llm.Response{Content: summary})
	a.model.gate = make(chan struct{}, 2)
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "POST", a.ts.URL+"/v1/notebooks/repo/runs", strings.NewReader(agentBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	_ = resp.Body.Close()
	a.model.gate <- struct{}{}
	a.model.gate <- struct{}{}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		code, body := call(t, a.ts, "GET", "/v1/outputs?notebook=repo", "")
		if code == 200 && strings.Contains(body, `"title"`) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the agent run did not finish after the client left")
}

func TestCancelHTTP(t *testing.T) {
	a := newAgentServer(t, grepCall())
	runID, _ := pausedRun(t, a)
	if code, body := call(t, a.ts, "POST", "/v1/runs/"+runID+"/cancel", ""); code != 204 {
		t.Fatalf("cancel: %d %s", code, body)
	}
	waitStatus(t, a, runID, notebook.StatusCancelled)
	if code, _ := call(t, a.ts, "POST", "/v1/runs/"+runID+"/cancel", ""); code != 409 {
		t.Fatalf("cancelling twice: %d", code)
	}
}
