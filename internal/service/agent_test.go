package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
)

// scriptModel replies from a queue; block, when set, holds every call until
// the context ends.
type scriptModel struct {
	mu      sync.Mutex
	replies []llm.Response
	block   bool
}

func (m *scriptModel) ChatStream(ctx context.Context, _ llm.Request, _ func(string)) (llm.Result, error) {
	if m.block {
		<-ctx.Done()
		return llm.Result{}, ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.replies) == 0 {
		return llm.Result{}, errors.New("no reply queued")
	}
	r := m.replies[0]
	m.replies = m.replies[1:]
	return llm.Result{Response: r, Target: "litellm/m", Attempts: []llm.Attempt{{Target: "litellm/m"}}}, nil
}

func askCall() llm.Response {
	return llm.Response{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "grep", Arguments: `{"pattern":"secret-arg"}`}}}
}

type agentEnv struct {
	svc    *Service
	model  *scriptModel
	nbID   string
	events []RunEvent
	mu     sync.Mutex
}

func newAgentEnv(t *testing.T, replies ...llm.Response) *agentEnv {
	t.Helper()
	e := &agentEnv{model: &scriptModel{replies: replies}}
	f := config.Default()
	f.Defaults.Model = "litellm/m"
	dir := t.TempDir()
	e.svc = &Service{
		Config: f, Secrets: secrets.Map{"TESSIVO_LITELLM_KEY": "k"}, Workspace: filepath.Join(dir, "ws"),
		UserAuditPath: filepath.Join(dir, "audit.jsonl"), Actor: "test",
		NewModel: func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return e.model, nil },
		OnRunEvent: func(ev RunEvent) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.events = append(e.events, ev)
		},
	}
	info, err := e.svc.CreateNotebook("Repo review", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(src, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.AddContext(info.ID, src); err != nil {
		t.Fatal(err)
	}
	e.nbID = info.ID
	return e
}

func (e *agentEnv) kinds() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.events))
	for _, ev := range e.events {
		out = append(out, ev.Kind)
	}
	return out
}

func (e *agentEnv) start(t *testing.T, ctx context.Context) RunOutcome {
	t.Helper()
	nb, err := e.svc.OpenNotebook(e.nbID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.svc.Run(ctx, RunRequest{
		Notebook: nb, Save: true, ToolRules: map[string]string{"grep": "ask"},
		Overrides: config.RunSettings{Mode: config.ModeAgent},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAgentRunPausesAndResumes(t *testing.T) {
	e := newAgentEnv(t, askCall(), llm.Response{Content: "done"}, llm.Response{Content: summary})
	out := e.start(t, context.Background())
	if len(out.Paused) != 1 || out.Paused[0].Tool != "grep" || out.Paused[0].ExpiresAt == "" {
		t.Fatalf("paused: %+v", out.Paused)
	}
	if k := e.kinds(); len(k) != 1 || k[0] != RunApprovalNeeded {
		t.Fatalf("events: %v", k)
	}
	pending, err := e.svc.Pending()
	if err != nil || len(pending) != 1 || pending[0].RunID != out.RunID {
		t.Fatalf("pending: %+v %v", pending, err)
	}
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", ""); err != nil {
		t.Fatal(err)
	}
	done, err := e.svc.Resume(context.Background(), out.RunID, nil)
	if err != nil || done.OutputID == "" || !done.Result.Valid {
		t.Fatalf("resume: %+v %v", done, err)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	run, err := nb.LoadRun(out.RunID)
	if err != nil || run.Status != notebook.StatusFinished || run.Mode != config.ModeAgent || len(run.ToolCalls) != 1 {
		t.Fatalf("run record: %+v %v", run, err)
	}
	if ids, _ := nb.ListCheckpoints(); len(ids) != 0 {
		t.Fatalf("checkpoint left behind: %v", ids)
	}
	log, err := os.ReadFile(filepath.Join(nb.Dir, AuditFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{`"tool.decision"`, `"tool.call"`, `"args_sha256"`, `"run.resume"`} {
		if !bytes.Contains(log, []byte(kind)) {
			t.Errorf("audit log lacks %s", kind)
		}
	}
	if bytes.Contains(log, []byte("secret-arg")) {
		t.Fatal("raw tool arguments leaked into the audit log")
	}
	if k := e.kinds(); k[len(k)-1] != RunFinished {
		t.Fatalf("events: %v", k)
	}
}

func checkpointBytes(t *testing.T, e *agentEnv, runID string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.svc.Workspace, e.nbID, notebook.RunsDir, runID+".checkpoint.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnswerErrors(t *testing.T) {
	e := newAgentEnv(t, askCall())
	out := e.start(t, context.Background())
	aid := out.Paused[0].ApprovalID
	before := checkpointBytes(t, e, out.RunID)
	cases := []struct {
		run, approval, answer string
		want                  error
	}{
		{"20990101T000000-000000", aid, "allow", ErrRunNotFound},
		{out.RunID, "nope", "allow", ErrApprovalNotFound},
		{out.RunID, aid, "maybe", nil},
	}
	for _, c := range cases {
		err := e.svc.Answer(c.run, c.approval, c.answer, "")
		if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
			t.Errorf("Answer(%s, %s, %s): want %v, got %v", c.run, c.approval, c.answer, c.want, err)
		}
	}
	if !bytes.Equal(before, checkpointBytes(t, e, out.RunID)) {
		t.Fatal("a failed answer changed the checkpoint")
	}
	if err := e.svc.Answer(out.RunID, aid, "deny", ""); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Answer(out.RunID, aid, "allow", ""); !errors.Is(err, ErrApprovalAnswered) {
		t.Fatalf("second answer: %v", err)
	}
}

func TestAnswerExpired(t *testing.T) {
	e := newAgentEnv(t, askCall())
	out := e.start(t, context.Background())
	e.svc.Now = func() time.Time { return time.Now().Add(8 * 24 * time.Hour) }
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", ""); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("want ErrApprovalExpired, got %v", err)
	}
}

func TestRememberNotebook(t *testing.T) {
	e := newAgentEnv(t, askCall())
	out := e.start(t, context.Background())
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", "notebook"); err != nil {
		t.Fatal(err)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	if nb.Manifest.Agent.Tools["grep"] != "allow" {
		t.Fatalf("notebook rules: %v", nb.Manifest.Agent.Tools)
	}
}

func TestResumeLocked(t *testing.T) {
	e := newAgentEnv(t, askCall())
	out := e.start(t, context.Background())
	nb, _ := e.svc.OpenNotebook(e.nbID)
	unlock, err := nb.LockRun(out.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := e.svc.Resume(context.Background(), out.RunID, nil); !errors.Is(err, notebook.ErrRunLocked) {
		t.Fatalf("want ErrRunLocked, got %v", err)
	}
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", ""); !errors.Is(err, notebook.ErrRunLocked) {
		t.Fatalf("answer while locked: want ErrRunLocked, got %v", err)
	}
}

func setStatus(t *testing.T, e *agentEnv, runID, status string, age time.Duration) {
	t.Helper()
	path := filepath.Join(e.svc.Workspace, e.nbID, notebook.RunsDir, runID+".checkpoint.json")
	var st map[string]any
	if err := json.Unmarshal(checkpointBytes(t, e, runID), &st); err != nil {
		t.Fatal(err)
	}
	cp := st["checkpoint"].(map[string]any)
	cp["status"] = status
	if age > 0 {
		for _, a := range cp["approvals"].([]any) {
			a.(map[string]any)["requestedAt"] = time.Now().Add(-age).UTC().Format(time.RFC3339)
		}
	}
	b, _ := json.Marshal(st)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRecover(t *testing.T) {
	e := newAgentEnv(t, askCall(), askCall())
	crashed := e.start(t, context.Background())
	setStatus(t, e, crashed.RunID, notebook.StatusRunning, 0)
	stale := e.start(t, context.Background())
	setStatus(t, e, stale.RunID, notebook.StatusWaiting, 8*24*time.Hour)
	if err := e.svc.Recover(); err != nil {
		t.Fatal(err)
	}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	if r, _ := nb.LoadRun(crashed.RunID); r.Status != notebook.StatusInterrupted {
		t.Fatalf("crashed run: %q", r.Status)
	}
	r, _ := nb.LoadRun(stale.RunID)
	if r.Status != notebook.StatusFailed || !strings.Contains(r.Error, "approval expired") {
		t.Fatalf("expired run: %+v", r)
	}
	if ids, _ := nb.ListCheckpoints(); len(ids) != 1 || ids[0] != crashed.RunID {
		t.Fatalf("checkpoints: %v", ids)
	}
}

func TestDetachedSurvivesCancel(t *testing.T) {
	e := newAgentEnv(t)
	e.model.block = true
	ctx, cancel := context.WithCancel(context.Background())
	nb, _ := e.svc.OpenNotebook(e.nbID)
	type result struct {
		out RunOutcome
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := e.svc.Run(ctx, RunRequest{
			Notebook: nb, Save: true, Detached: true, Overrides: config.RunSettings{Mode: config.ModeAgent},
		}, nil)
		done <- result{out, err}
	}()
	runID := waitForJob(t, e.svc)
	cancel()
	select {
	case <-done:
		t.Fatal("a detached run must outlive its caller's context")
	case <-time.After(100 * time.Millisecond):
	}
	if err := e.svc.Cancel(runID); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err == nil || !errors.Is(r.err, context.Canceled) {
		t.Fatalf("cancelled run: %v", r.err)
	}
	if run, _ := nb.LoadRun(runID); run.Status != notebook.StatusCancelled {
		t.Fatalf("status %q", run.Status)
	}
}

func waitForJob(t *testing.T, s *Service) string {
	t.Helper()
	for range 200 {
		if ids := s.liveRuns(); len(ids) == 1 {
			return ids[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no live run appeared")
	return ""
}

func TestAgentNeedsNotebook(t *testing.T) {
	e := newAgentEnv(t)
	_, err := e.svc.Run(context.Background(), RunRequest{
		WorkType: "summarize", Overrides: config.RunSettings{Mode: config.ModeAgent},
	}, nil)
	if !IsInvalid(err) {
		t.Fatalf("want an invalid-request error, got %v", err)
	}
}

func TestResumeCheckAndStatus(t *testing.T) {
	e := newAgentEnv(t, askCall(), llm.Response{Content: "done"}, llm.Response{Content: summary})
	out := e.start(t, context.Background())
	if err := e.svc.ResumeCheck(out.RunID); !errors.Is(err, ErrApprovalPending) {
		t.Fatalf("unanswered: %v", err)
	}
	if st, err := e.svc.RunStatus(out.RunID); err != nil || st != notebook.StatusWaiting {
		t.Fatalf("status: %q %v", st, err)
	}
	if err := e.svc.Answer(out.RunID, out.Paused[0].ApprovalID, "allow", "run"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ResumeCheck(out.RunID); err != nil {
		t.Fatalf("answered: %v", err)
	}
	if _, err := e.svc.Resume(context.Background(), out.RunID, nil); err != nil {
		t.Fatal(err)
	}
	if st, err := e.svc.RunStatus(out.RunID); err != nil || st != notebook.StatusFinished {
		t.Fatalf("status after resume: %q %v", st, err)
	}
	if err := e.svc.ResumeCheck(out.RunID); !errors.Is(err, ErrRunNotPaused) {
		t.Fatalf("finished run: %v", err)
	}
	if _, err := e.svc.RunStatus("nope"); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("unknown run: %v", err)
	}
}

func TestAgentRedactsKnownSecrets(t *testing.T) {
	const key = "lk-9f8e7d6c5b4a3210" // a configured key: not shaped like sk-…, so only value redaction finds it
	e := newAgentEnv(t,
		llm.Response{ToolCalls: []llm.ToolCall{{ID: "c0", Name: "read_file", Arguments: `{"path":"a.go"}`}}},
		askCall(),
	)
	e.svc.Secrets = secrets.Map{"TESSIVO_LITELLM_KEY": key}
	nb, _ := e.svc.OpenNotebook(e.nbID)
	src := nb.ContextPath(nb.Manifest.Context[0])
	if err := os.WriteFile(filepath.Join(src, "a.go"), []byte("package a // key "+key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := e.start(t, context.Background())
	if len(out.Paused) != 1 {
		t.Fatalf("want a pause, got %+v", out)
	}
	for _, name := range []string{out.RunID + ".checkpoint.json", out.RunID + ".json"} {
		b, err := os.ReadFile(filepath.Join(nb.Dir, notebook.RunsDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(key)) {
			t.Fatalf("%s holds the configured key", name)
		}
		if !bytes.Contains(b, []byte("[REDACTED]")) {
			t.Fatalf("%s: the tool result should be kept, redacted", name)
		}
	}
}
