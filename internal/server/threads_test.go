package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

const seedSummary = `{"title":"Board digest","tldr":"t","key_points":["k"],"per_source":[],"action_items":[]}`

type reply struct {
	text  string
	calls []llm.ToolCall
}

// queued answers in order ("ok" when empty); hold, when set, blocks the
// first request until release is closed.
type queued struct {
	mu      sync.Mutex
	replies []reply
	hold    chan struct{}
	release chan struct{}
}

func (q *queued) ChatStream(ctx context.Context, _ llm.Request, onDelta func(string)) (llm.Result, error) {
	q.mu.Lock()
	r := reply{text: "ok"}
	if len(q.replies) > 0 {
		r, q.replies = q.replies[0], q.replies[1:]
	}
	hold := q.hold
	q.hold = nil
	q.mu.Unlock()
	if hold != nil {
		close(hold)
		select {
		case <-q.release:
		case <-ctx.Done():
			return llm.Result{}, ctx.Err()
		}
	}
	if r.text != "" && onDelta != nil {
		onDelta(r.text)
	}
	return llm.Result{Response: llm.Response{Content: r.text, ToolCalls: r.calls}, Target: "litellm/m"}, nil
}

// threadServer has one notebook with one output; base is the output's path.
func threadServer(t *testing.T, replies ...reply) (*testServerT, *queued) {
	t.Helper()
	ts, s := newTestServer(t, seedSummary)
	info, err := s.svc.CreateNotebook("Board", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	nb, _ := s.svc.OpenNotebook(info.ID)
	out, err := s.svc.Run(context.Background(),
		service.RunRequest{Notebook: nb, Prompt: "Summarize the board.", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	q := &queued{replies: replies}
	s.svc.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return q, nil }
	return &testServerT{
		ts: ts, s: s, nb: nb, base: "/v1/notebooks/" + info.ID + "/outputs/" + out.OutputID,
		output: out.OutputID,
	}, q
}

func TestThreadMessageStreams(t *testing.T) {
	ts, _ := threadServer(t,
		reply{text: "Noted.", calls: []llm.ToolCall{
			{ID: "c1", Name: "suggest_memory", Arguments: `{"text":"cite files"}`},
		}},
		reply{text: "Done."})
	code, body := call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"always cite files"}`)
	ev := sseEvents(body)
	kinds := make([]string, 0, len(ev))
	for _, e := range ev {
		kinds = append(kinds, e[0])
	}
	if code != 200 || strings.Join(kinds, ",") != "delta,suggestion,delta,finished" {
		t.Fatalf("send: %d %v\n%s", code, kinds, body)
	}
	var fin service.ThreadReply
	if err := json.Unmarshal([]byte(ev[3][1]), &fin); err != nil || fin.Assistant.Content != "Noted.\n\nDone." ||
		len(fin.Suggestions) != 1 {
		t.Fatalf("finished: %v %+v", err, fin)
	}
	code, body = call(t, ts.ts, "GET", ts.base+"/thread", "")
	if code != 200 || !strings.Contains(body, "always cite files") {
		t.Fatalf("get thread: %d %s", code, body)
	}
	code, body = call(t, ts.ts, "PATCH", ts.base+"/thread", `{"thinkingEffort":"high"}`)
	if code != 200 || !strings.Contains(body, `"thinkingEffort":"high"`) {
		t.Fatalf("patch thread: %d %s", code, body)
	}
	if code, _ = call(t, ts.ts, "PATCH", ts.base+"/thread", `{"thinkingEffort":"wild"}`); code != 400 {
		t.Fatalf("bad effort: %d", code)
	}
}

func TestThreadBusy409(t *testing.T) {
	ts, q := threadServer(t)
	q.hold, q.release = make(chan struct{}), make(chan struct{})
	hold := q.hold
	done := make(chan struct{})
	go func() {
		defer close(done)
		call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"first"}`)
	}()
	<-hold
	_, body := call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"second"}`)
	ev := sseEvents(body)
	if len(ev) != 1 || ev[0][0] != "error" || !strings.Contains(ev[0][1], `"status":409`) {
		t.Fatalf("busy: %s", body)
	}
	if code, _ := call(t, ts.ts, "POST", ts.base+"/versions", `{}`); code != 409 {
		t.Fatalf("version while busy: %d", code)
	}
	close(q.release)
	<-done
}

func TestSuggestionApplyAndDismiss(t *testing.T) {
	ts, _ := threadServer(t, reply{calls: []llm.ToolCall{
		{ID: "a", Name: "suggest_memory", Arguments: `{"text":"cite files"}`},
		{ID: "b", Name: "suggest_prompt", Arguments: `{"text":"Summarize the board for {{team}}."}`},
	}})
	call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"q"}`)
	code, body := call(t, ts.ts, "GET", ts.base+"/suggestions", "")
	var list struct {
		Suggestions []service.SuggestionInfo `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(body), &list); code != 200 || err != nil || len(list.Suggestions) != 2 {
		t.Fatalf("list: %d %s", code, body)
	}
	mem, prompt := list.Suggestions[0], list.Suggestions[1]
	if prompt.Target == nil || prompt.Target.Kind != "new" {
		t.Fatalf("target: %+v", prompt.Target)
	}
	if code, body = call(t, ts.ts, "POST", ts.base+"/suggestions/"+prompt.ID+"/apply", ""); code != 400 {
		t.Fatalf("new variable: %d %s", code, body)
	}
	if code, body = call(t, ts.ts, "POST", ts.base+"/suggestions/"+mem.ID+"/apply", ""); code != 200 ||
		!strings.Contains(body, `"status":"applied"`) {
		t.Fatalf("apply: %d %s", code, body)
	}
	if code, _ = call(t, ts.ts, "POST", ts.base+"/suggestions/"+mem.ID+"/apply", ""); code != 409 {
		t.Fatalf("apply twice: %d", code)
	}
	if code, body = call(t, ts.ts, "POST", ts.base+"/suggestions/"+prompt.ID+"/dismiss", ""); code != 200 ||
		!strings.Contains(body, `"status":"dismissed"`) {
		t.Fatalf("dismiss: %d %s", code, body)
	}
	if code, _ = call(t, ts.ts, "POST", ts.base+"/suggestions/"+notebook.NewID()+"/dismiss", ""); code != 404 {
		t.Fatalf("unknown suggestion: %d", code)
	}
}

func TestApplyStale409(t *testing.T) {
	ts, _ := threadServer(t, reply{calls: []llm.ToolCall{
		{ID: "a", Name: "suggest_prompt", Arguments: `{"text":"Summarize the board. Cite files."}`},
	}})
	call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"q"}`)
	_, body := call(t, ts.ts, "GET", ts.base+"/suggestions", "")
	var list struct {
		Suggestions []service.SuggestionInfo `json:"suggestions"`
	}
	_ = json.Unmarshal([]byte(body), &list)
	runs, _ := ts.nb.ListOutputs()
	run, _ := ts.nb.LoadRun(runs[0].RunID)
	run.Prompt = "Edited after the suggestion."
	_ = ts.nb.SaveRun(run)
	if code, body := call(t, ts.ts, "POST", ts.base+"/suggestions/"+list.Suggestions[0].ID+"/apply", ""); code != 409 {
		t.Fatalf("stale: %d %s", code, body)
	}
}

func TestGenerateSuggestionsAPI(t *testing.T) {
	ts, q := threadServer(t)
	call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"always cite files"}`)
	q.replies = []reply{{text: `{"suggestions":[{"kind":"memory","text":"cite files"}]}`}}
	code, body := call(t, ts.ts, "POST", ts.base+"/suggestions/generate", "")
	if code != 200 || !strings.Contains(body, "cite files") {
		t.Fatalf("generate: %d %s", code, body)
	}
}

func TestVersions201And422(t *testing.T) {
	ts, q := threadServer(t)
	call(t, ts.ts, "POST", ts.base+"/thread/messages", `{"text":"shorter please"}`)
	q.replies = []reply{{text: "nope"}, {text: "nope"}, {text: "nope"}}
	if code, body := call(t, ts.ts, "POST", ts.base+"/versions", `{}`); code != 422 {
		t.Fatalf("invalid version: %d %s", code, body)
	}
	q.replies = []reply{{text: seedSummary}}
	code, body := call(t, ts.ts, "POST", ts.base+"/versions", `{}`)
	if code != 201 || !strings.Contains(body, `"parentId":"`+ts.output+`"`) || !strings.Contains(body, `"version":2`) {
		t.Fatalf("version: %d %s", code, body)
	}
	code, body = call(t, ts.ts, "GET", "/v1/outputs", "")
	if code != 200 || !strings.Contains(body, `"parentId"`) || !strings.Contains(body, `"version":2`) {
		t.Fatalf("outputs list: %d %s", code, body)
	}
}

func TestThreadUnknownOutput404(t *testing.T) {
	ts, _ := threadServer(t)
	bad := strings.Replace(ts.base, ts.output, notebook.NewID(), 1)
	for _, c := range [][2]string{{"GET", "/thread"}, {"GET", "/suggestions"}, {"POST", "/versions"}} {
		if code, body := call(t, ts.ts, c[0], bad+c[1], "{}"); code != 404 {
			t.Errorf("%s %s: %d %s", c[0], c[1], code, body)
		}
	}
	if code, _ := call(t, ts.ts, "POST", bad+"/thread/messages", `{"text":"q"}`); code != 404 {
		t.Errorf("message: %d", code)
	}
}

type testServerT struct {
	ts     *httptest.Server
	s      *Server
	nb     *notebook.Notebook
	base   string
	output string
}
