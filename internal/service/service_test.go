package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

type fakeModel struct{ reply string }

func (f fakeModel) ChatStream(_ context.Context, _ llm.Request, onDelta func(string)) (llm.Result, error) {
	if onDelta != nil {
		onDelta(f.reply)
	}
	return llm.Result{
		Response: llm.Response{Content: f.reply}, Target: "litellm/m",
		Attempts: []llm.Attempt{{Target: "litellm/m"}},
	}, nil
}

const summary = `{"title":"Board digest","tldr":"t","key_points":["k"],"per_source":[],"action_items":[]}`

func newSvc(t *testing.T, reply string) *Service {
	t.Helper()
	f := config.Default()
	f.Defaults.Model = "litellm/m"
	dir := t.TempDir()
	return &Service{
		Config: f, Secrets: secrets.Map{"TESSIVO_LITELLM_KEY": "k"}, Workspace: filepath.Join(dir, "ws"),
		UserAuditPath: filepath.Join(dir, "audit.jsonl"), Actor: "test",
		NewModel: func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
			return fakeModel{reply}, nil
		},
	}
}

func TestNotebookLifecycle(t *testing.T) {
	s := newSvc(t, summary)
	if list, err := s.ListNotebooks(); err != nil || len(list) != 0 {
		t.Fatalf("empty workspace: %v %v", list, err)
	}
	a, err := s.CreateNotebook("Board Minutes!", "summarize")
	if err != nil || a.ID != "board-minutes" {
		t.Fatalf("%v %+v", err, a)
	}
	b, _ := s.CreateNotebook("board minutes", "summarize")
	if b.ID != "board-minutes-2" {
		t.Fatalf("want unique id, got %s", b.ID)
	}
	if _, err := s.CreateNotebook(" ", "summarize"); !IsInvalid(err) {
		t.Fatal("want invalid name")
	}
	if _, err := s.CreateNotebook("x", "poetry"); !IsInvalid(err) {
		t.Fatalf("want invalid work type, got %v", err)
	}
	_ = os.MkdirAll(filepath.Join(s.Workspace, "not-a-notebook"), 0o700)
	list, _ := s.ListNotebooks()
	if len(list) != 2 {
		t.Fatalf("want 2 notebooks, got %d", len(list))
	}
	for _, bad := range []string{"../etc", "A", "", "x/y", strings.Repeat("a", 70)} {
		if _, err := s.OpenNotebook(bad); !IsInvalid(err) {
			t.Errorf("%q: want invalid id, got %v", bad, err)
		}
	}
	if _, err := s.Describe("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}

func TestContextAndRun(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Board", "summarize")
	src := filepath.Join(t.TempDir(), "minutes.md")
	_ = os.WriteFile(src, []byte("Decided to ship in March."), 0o600)
	if _, err := s.AddContext(info.ID, "relative.md"); !IsInvalid(err) {
		t.Fatal("want absolute path required")
	}
	if _, err := s.AddContext(info.ID, src+".missing"); !IsInvalid(err) {
		t.Fatal("want missing file error")
	}
	got, err := s.AddContext(info.ID, src)
	if err != nil || len(got.Context) != 1 || got.Context[0].EstimatedTokens == 0 {
		t.Fatalf("%v %+v", err, got)
	}
	ctxID := got.Context[0].ID
	if _, err := s.SetContextEnabled(info.ID, "nope", false); !errors.Is(err, ErrNotFound) {
		t.Fatal("want not found")
	}

	nb, _ := s.OpenNotebook(info.ID)
	var deltas int
	out, err := s.Run(context.Background(), RunRequest{
		Notebook: nb, Save: true,
		Extra: []extract.Document{{Source: "stdin", Kind: "text", Text: "extra"}},
	}, func(e executor.Event) {
		if e.Kind == executor.EventDelta {
			deltas++
		}
	})
	if err != nil || out.OutputID == "" || !strings.Contains(out.Markdown, "# Board digest") || deltas != 1 {
		t.Fatalf("%v %+v", err, out)
	}
	if len(out.Spec.Documents) != 2 {
		t.Fatalf("want notebook + extra docs, got %d", len(out.Spec.Documents))
	}
	if _, err := s.SetContextEnabled(info.ID, ctxID, false); err != nil {
		t.Fatal(err)
	}

	outs, _ := s.ListOutputs("", "summarize")
	if len(outs) != 1 || outs[0].NotebookName != "Board" || outs[0].Data != nil {
		t.Fatalf("outputs: %+v", outs)
	}
	if o, _ := s.ListOutputs("", "report"); len(o) != 0 {
		t.Fatal("filter by work type")
	}
	full, err := s.GetOutput(info.ID, out.OutputID)
	if err != nil || !strings.Contains(full.Markdown, "Board digest") || len(full.Data) == 0 {
		t.Fatalf("%v %+v", err, full)
	}
	if _, err := s.GetOutput(info.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatal("want not found")
	}
	if b, _ := os.ReadFile(filepath.Join(nb.Dir, AuditFile)); !strings.Contains(string(b), `"actor":"test"`) {
		t.Fatal("run not audited with actor")
	}
}

func TestRunValidation(t *testing.T) {
	s := newSvc(t, summary)
	if _, err := s.Run(context.Background(), RunRequest{}, nil); !IsInvalid(err) {
		t.Fatal("want work type required")
	}
	if _, err := s.Run(context.Background(), RunRequest{WorkType: "x"}, nil); !IsInvalid(err) {
		t.Fatal("want unknown work type")
	}
	bad := RunRequest{WorkType: "summarize", Overrides: config.RunSettings{ThinkingEffort: "max"}}
	if _, err := s.Run(context.Background(), bad, nil); !IsInvalid(err) {
		t.Fatal("want invalid overrides")
	}
	ghost := RunRequest{WorkType: "summarize", Overrides: config.RunSettings{Model: "ghost"}}
	if _, err := s.Run(context.Background(), ghost, nil); !IsInvalid(err) {
		t.Fatal("want unknown model")
	}
	s.Config.Defaults.Model = ""
	if _, err := s.Run(context.Background(), RunRequest{WorkType: "summarize"}, nil); !IsInvalid(err) {
		t.Fatal("want model required")
	}
	s = newSvc(t, "not json")
	if _, err := s.Run(context.Background(), RunRequest{WorkType: "summarize"}, nil); !errors.Is(err,
		llm.ErrInvalidOutput) {
		t.Fatalf("want invalid output, got %v", err)
	}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
		return nil, errors.New("no key")
	}
	if _, err := s.Run(context.Background(), RunRequest{WorkType: "summarize"}, nil); err == nil {
		t.Fatal("want factory error")
	}
}

func TestModelsAndSettings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"id":"qwen"}]}`)
	}))
	t.Cleanup(srv.Close)
	s := newSvc(t, summary)
	s.Config.Providers[0].BaseURL = srv.URL
	ms, err := s.Models(context.Background(), "")
	if err != nil || ms["litellm"][0] != "qwen" {
		t.Fatalf("%v %v", err, ms)
	}
	if ms, _ := s.Models(context.Background(), "other"); len(ms) != 0 {
		t.Fatal("provider filter")
	}
	s.Secrets = secrets.Map{}
	if _, err := s.Models(context.Background(), ""); err == nil {
		t.Fatal("want missing key")
	}
	s.Config.Providers[0].APIKeyRef = ""
	s.Config.Providers[0].BaseURL = "http://127.0.0.1:1"
	if _, err := s.Models(context.Background(), ""); err == nil {
		t.Fatal("want transport error")
	}
	if s.Settings().Defaults.Model != "litellm/m" || s.AuditPath(nil) != s.UserAuditPath {
		t.Fatal("settings and audit path")
	}
	if Slug("!!!") != "notebook" || Slug(strings.Repeat("ab ", 40)) != strings.Repeat("ab-", 16)[:47] {
		t.Fatalf("slug: %q", Slug(strings.Repeat("ab ", 40)))
	}
	if _, err := DefaultModelFactory(s.Config, config.Alias{Name: "a", Chain: []config.ModelRef{{
		Provider: "litellm",
		Model:    "m",
	}}}, secrets.Map{}); err != nil {
		t.Fatal(err)
	}
}
