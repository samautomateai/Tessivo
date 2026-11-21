package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/testutil"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
)

type fakeModel struct {
	reply string
	err   error
	reqs  []llm.Request
}

func (f *fakeModel) ChatStream(_ context.Context, req llm.Request, _ func(string)) (llm.Result, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return llm.Result{}, f.err
	}
	return llm.Result{
		Response: llm.Response{Content: f.reply, Usage: llm.Usage{TotalTokens: 5}},
		Target:   "litellm/m", Attempts: []llm.Attempt{{Target: "litellm/m"}, {Target: "x", Err: errors.New("e")}},
	}, nil
}

const summary = `{"title":"Digest","tldr":"short","key_points":["one"],"per_source":[],"action_items":["do"]}`

type harness struct {
	t      *testing.T
	env    Env
	out    *bytes.Buffer
	errOut *bytes.Buffer
	model  *fakeModel
	dir    string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	h := &harness{t: t, out: &bytes.Buffer{}, errOut: &bytes.Buffer{}, model: &fakeModel{reply: summary}, dir: dir}
	h.env = Env{
		Stdin: strings.NewReader(""), StdinIsTTY: true, Stdout: h.out, Stderr: h.errOut,
		ConfigPath: filepath.Join(dir, "config.json"), Secrets: secrets.Map{"TESSIVO_LITELLM_KEY": "k"},
		NewModel: func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return h.model, nil },
		ListModels: func(context.Context, config.Provider, secrets.Resolver) ([]llm.Model, error) {
			return []llm.Model{{ID: "qwen"}}, nil
		},
	}
	return h
}

func (h *harness) run(args ...string) int {
	h.t.Helper()
	h.out.Reset()
	h.errOut.Reset()
	return Main(context.Background(), args, h.env)
}

func (h *harness) mustRun(args ...string) string {
	h.t.Helper()
	if code := h.run(args...); code != ExitOK {
		h.t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, code, h.out, h.errOut)
	}
	return h.out.String()
}

func (h *harness) initConfig() {
	h.t.Helper()
	h.mustRun("config", "init")
	f, _ := config.Load(h.env.ConfigPath)
	f.Defaults.Model = "litellm/m"
	if err := f.Save(h.env.ConfigPath); err != nil {
		h.t.Fatal(err)
	}
}

func TestVersionHelpAndUsage(t *testing.T) {
	h := newHarness(t)
	if out := h.mustRun("version"); !strings.Contains(out, "tess dev") {
		t.Fatal(out)
	}
	if out := h.mustRun("help"); !strings.Contains(out, "Usage:") {
		t.Fatal(out)
	}
	for _, args := range [][]string{
		{},
		{"bogus"},
		{"new"},
		{"context", "x"},
		{"config"},
		{"config", "x"},
		{"outputs"},
		{"run", "a", "b"},
		{"new", "x", "--nope"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage exit, got %d (%s)", args, code, h.errOut)
		}
	}
	if code := h.run("run", "-h"); code != ExitOK {
		t.Fatalf("help flag: %d", code)
	}
}

func TestConfigCommands(t *testing.T) {
	h := newHarness(t)
	if code := h.run("config", "show"); code != ExitError || !strings.Contains(h.errOut.String(), "config init") {
		t.Fatalf("missing config: %d %s", code, h.errOut)
	}
	h.mustRun("config", "init")
	if code := h.run("config", "init"); code != ExitError {
		t.Fatal("want refusal without --force")
	}
	h.mustRun("config", "init", "--force")
	if out := h.mustRun("config", "path"); strings.TrimSpace(out) != h.env.ConfigPath {
		t.Fatal(out)
	}
	shown := h.mustRun("config", "show")
	if out := shown; !strings.Contains(out, "TESSIVO_LITELLM_KEY") || strings.Contains(out, `"k"`) {
		t.Fatalf("show must list the key ref, never the key: %s", out)
	}
	if out := h.mustRun("models"); !strings.Contains(out, "litellm") || !strings.Contains(out, "qwen") {
		t.Fatal(out)
	}
	if out := h.mustRun("models", "--provider", "other"); strings.Contains(out, "qwen") {
		t.Fatal(out)
	}
	h.env.ListModels = func(context.Context, config.Provider, secrets.Resolver) ([]llm.Model, error) {
		return nil, errors.New("down")
	}
	if code := h.run("models"); code != ExitError {
		t.Fatal("want provider error")
	}
	if out := h.mustRun("worktypes"); !strings.Contains(out, "summarize") {
		t.Fatal(out)
	}
}

func TestNotebookFlow(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	nbDir := filepath.Join(h.dir, "vendors")
	h.mustRun("new", nbDir, "--type", "summarize", "--name", "Vendors")
	src := filepath.Join(h.dir, "contract.md")
	_ = os.WriteFile(src, []byte("Renewal in March. Price 10k."), 0o600)
	codeDir := filepath.Join(h.dir, "code")
	_ = os.MkdirAll(codeDir, 0o750)
	_ = os.WriteFile(filepath.Join(codeDir, "main.go"), []byte("package main"), 0o600)
	h.mustRun("context", "add", nbDir, src, codeDir)
	out := h.mustRun("context", "list", nbDir)
	if !strings.Contains(out, "contract.md") || !strings.Contains(out, "code") {
		t.Fatal(out)
	}
	h.mustRun("context", "disable", nbDir, "code")

	out = h.mustRun("run", nbDir, "--prompt", "Summarize for {{who}}", "--var", "who=legal",
		"--thinking", "high", "--quiet")
	if !strings.Contains(out, "# Digest") || !strings.Contains(out, "## TL;DR") {
		t.Fatalf("markdown: %s", out)
	}
	req := h.model.reqs[0]
	user := req.Messages[1].Content
	if req.ReasoningEffort != "high" || !strings.Contains(user, "Summarize for legal") ||
		!strings.Contains(user, "Renewal in March") || strings.Contains(user, "main.go") {
		t.Fatalf("request: %+v", user)
	}
	if out := h.mustRun("outputs", nbDir); !strings.Contains(out, "Digest") {
		t.Fatal(out)
	}
	runs, _ := os.ReadDir(filepath.Join(nbDir, notebook.RunsDir))
	if len(runs) != 1 {
		t.Fatalf("want 1 run record, got %d", len(runs))
	}
	jsonOut := filepath.Join(h.dir, "out.json")
	h.mustRun("run", nbDir, "--json", "--out", jsonOut, "--no-save")
	if b, _ := os.ReadFile(jsonOut); !strings.Contains(string(b), `"tldr"`) {
		t.Fatal("json output not written")
	}
	nb, _ := notebook.Open(nbDir)
	legal := notebook.Prompt{Name: "legal", Text: "Legal summary", Settings: config.RunSettings{ThinkingEffort: "low"}}
	_ = nb.SavePrompt(legal)
	h.mustRun("run", nbDir, "--prompt-name", "legal")
	if h.model.reqs[len(h.model.reqs)-1].ReasoningEffort != "low" {
		t.Fatal("prompt-level settings not applied")
	}
	if code := h.run("run", nbDir, "--prompt-name", "missing"); code != ExitError {
		t.Fatal("want missing prompt error")
	}
	if code := h.run("context", "enable", nbDir); code != ExitUsage {
		t.Fatal("want usage error")
	}
	if code := h.run("context", "zap", nbDir); code != ExitUsage {
		t.Fatal("want usage error")
	}
	if code := h.run("context", "add", nbDir); code != ExitUsage {
		t.Fatal("want usage error")
	}
}

func TestRunFromStdinAndErrors(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader("meeting notes: ship friday"), false
	out := h.mustRun("run", "--type", "summarize", "-")
	if !strings.Contains(out, "# Digest") || !strings.Contains(h.model.reqs[0].Messages[1].Content, "ship friday") {
		t.Fatalf("stdin run: %s", out)
	}
	h.env.Stdin, h.env.StdinIsTTY = strings.NewReader(""), true
	if code := h.run("run"); code != ExitUsage {
		t.Fatal("want usage error without notebook or type")
	}
	if code := h.run("run", "--type", "poem"); code != ExitUsage {
		t.Fatal("want unknown work type")
	}
	if code := h.run("run", "--type", "summarize", "--thinking", "max"); code != ExitUsage {
		t.Fatal("want invalid thinking")
	}
	if code := h.run("run", "--type", "summarize", "--var", "novalue"); code != ExitUsage {
		t.Fatal("want invalid var")
	}
	if code := h.run("run", "--type", "summarize", "--model", "ghost"); code != ExitUsage {
		t.Fatal("want unknown model")
	}
	if code := h.run("run", filepath.Join(h.dir, "nope")); code != ExitError {
		t.Fatal("want missing notebook")
	}
	h.model.reply = "not json"
	if code := h.run("run", "--type", "summarize"); code != ExitInvalidOutput {
		t.Fatalf("want invalid output exit, got %d", code)
	}
	h.env.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
		return nil, errors.New("no key")
	}
	if code := h.run("run", "--type", "summarize"); code != ExitError {
		t.Fatal("want model build error")
	}
}

func TestRunWithoutDefaultModel(t *testing.T) {
	h := newHarness(t)
	h.mustRun("config", "init")
	if code := h.run("run", "--type", "summarize"); code != ExitUsage {
		t.Fatalf("want usage error, got %d", code)
	}
	out := testutil.Unwritable(t, "y")
	if code := h.run("run", "--type", "summarize", "--model", "litellm/m", "--out", out); code != ExitError {
		t.Fatal("want write error")
	}
}

func TestDefaults(t *testing.T) {
	env := withDefaults(Env{})
	if env.Secrets == nil || env.NewModel == nil || env.ListModels == nil {
		t.Fatal("defaults not set")
	}
	f := config.Default()
	if _, err := env.NewModel(f, config.Alias{Name: "x", Chain: []config.ModelRef{{Provider: "litellm", Model: "m"}}},
		secrets.Map{"TESSIVO_LITELLM_KEY": "k"}); err != nil {
		t.Fatal(err)
	}
	noKey := config.Provider{Name: "p", APIKeyRef: "MISSING_X"}
	if _, err := listModels(context.Background(), noKey, secrets.Map{}); err == nil {
		t.Fatal("want missing key")
	}
	if _, err := listModels(context.Background(), config.Provider{Name: "p", BaseURL: "http://127.0.0.1:1"},
		secrets.Map{}); err == nil {
		t.Fatal("want transport error")
	}
}
