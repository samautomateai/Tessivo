package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// outputHarness has a notebook folder with one saved output.
func outputHarness(t *testing.T) (h *harness, dir, outputID string) {
	t.Helper()
	h = newHarness(t)
	h.initConfig()
	dir = filepath.Join(h.dir, "board")
	h.mustRun("new", dir, "--type", "summarize")
	h.mustRun("run", dir, "--prompt", "Summarize the board.")
	nb, err := notebook.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	outs, err := nb.ListOutputs()
	if err != nil || len(outs) != 1 {
		t.Fatalf("seed output: %v %d", err, len(outs))
	}
	return h, dir, outs[0].ID
}

var suggestionLine = regexp.MustCompile(`suggestion (\S+) memory: cite files`)

func TestOutputAskPrintsSuggestions(t *testing.T) {
	h, dir, out := outputHarness(t)
	h.model.calls = []llm.ToolCall{{ID: "c", Name: "suggest_memory", Arguments: `{"text":"cite files"}`}}
	h.model.reply = "Noted."
	got := h.mustRun("output", "ask", dir, out, "always", "cite", "files")
	if !strings.Contains(got, "Noted.") || !suggestionLine.MatchString(got) {
		t.Fatalf("ask: %q", got)
	}
	if list := h.mustRun("output", "suggestions", dir, out); !strings.Contains(list, "open") ||
		!strings.Contains(list, "cite files") {
		t.Fatalf("suggestions: %q", list)
	}
}

func TestOutputApplyAndDismiss(t *testing.T) {
	h, dir, out := outputHarness(t)
	h.model.calls = []llm.ToolCall{
		{ID: "a", Name: "suggest_memory", Arguments: `{"text":"cite files"}`},
		{ID: "b", Name: "suggest_memory", Arguments: `{"text":"be brief"}`},
	}
	h.mustRun("output", "ask", dir, out, "q")
	nb, _ := notebook.Open(dir)
	list, _ := nb.Suggestions(out)
	h.mustRun("output", "apply", dir, out, list[0].ID)
	h.mustRun("output", "dismiss", dir, out, list[1].ID)
	nb, _ = notebook.Open(dir)
	if mem := nb.Manifest.Memory; len(mem) != 1 || mem[0] != "cite files" {
		t.Fatalf("memory %v", mem)
	}
	if code := h.run("output", "apply", dir, out, list[0].ID); code == ExitOK {
		t.Fatal("applying twice must fail")
	}
}

func TestOutputSuggest(t *testing.T) {
	h, dir, out := outputHarness(t)
	h.model.reply = "Sure."
	h.mustRun("output", "ask", dir, out, "always", "cite", "files")
	h.model.reply = `{"suggestions":[{"kind":"memory","text":"cite files"}]}`
	if got := h.mustRun("output", "suggest", dir, out); !suggestionLine.MatchString(got) {
		t.Fatalf("suggest: %q", got)
	}
}

func TestOutputRefinePrintsVersion(t *testing.T) {
	h, dir, out := outputHarness(t)
	h.model.reply = "I will shorten it."
	h.mustRun("output", "ask", dir, out, "shorter")
	h.model.reply = summary
	got := h.mustRun("output", "refine", dir, out)
	if !regexp.MustCompile(`saved v2 \d{8}T\d{6}-[0-9a-f]{6}`).MatchString(got) {
		t.Fatalf("refine: %q", got)
	}
}

func TestOutputUsage(t *testing.T) {
	h, dir, out := outputHarness(t)
	for _, args := range [][]string{
		{"output"},
		{"output", "bogus"},
		{"output", "ask", dir},
		{"output", "ask", dir, out},
		{"output", "apply", dir, out},
		{"output", "refine", dir, out, "--nope"},
	} {
		if code := h.run(args...); code != ExitUsage {
			t.Errorf("%v: want usage exit, got %d (%s)", args, code, h.errOut)
		}
	}
}
