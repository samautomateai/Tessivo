package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// step is one scripted model answer.
type step struct {
	text  string
	calls []llm.ToolCall
	err   error
}

// scripted answers requests in order and records them. When it runs out of
// steps it answers "ok". hold, when set, is closed on the first request and
// the model then waits for release.
type scripted struct {
	mu      sync.Mutex
	steps   []step
	reqs    []llm.Request
	hold    chan struct{}
	release chan struct{}
}

func (m *scripted) ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	m.mu.Lock()
	m.reqs = append(m.reqs, req)
	st := step{text: "ok"}
	if len(m.steps) > 0 {
		st, m.steps = m.steps[0], m.steps[1:]
	}
	hold := m.hold
	m.hold = nil
	m.mu.Unlock()
	if hold != nil {
		close(hold)
		select {
		case <-m.release:
		case <-ctx.Done():
			return llm.Result{}, ctx.Err()
		}
	}
	if st.err != nil {
		return llm.Result{}, st.err
	}
	if onDelta != nil && st.text != "" {
		onDelta(st.text)
	}
	return llm.Result{Response: llm.Response{Content: st.text, ToolCalls: st.calls}, Target: "litellm/m"}, nil
}

func (m *scripted) requests() []llm.Request {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]llm.Request(nil), m.reqs...)
}

// threadFixture is a notebook with one saved output, run by fakeModel.
type threadFixture struct {
	s      *Service
	nbID   string
	nb     *notebook.Notebook
	output string
	model  *scripted
}

func newThreadFixture(t *testing.T, steps ...step) threadFixture {
	t.Helper()
	s := newSvc(t, summary)
	info, err := s.CreateNotebook("Board", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetMemory(info.ID, []string{"notebook fact: board meets monthly"}); err != nil {
		t.Fatal(err)
	}
	nb, _ := s.OpenNotebook(info.ID)
	out, err := s.Run(context.Background(), RunRequest{Notebook: nb, Prompt: "Summarize the board.", Save: true}, nil)
	if err != nil || out.OutputID == "" {
		t.Fatalf("seed run: %v %+v", err, out)
	}
	m := &scripted{steps: steps}
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	nb, _ = s.OpenNotebook(info.ID)
	return threadFixture{s: s, nbID: info.ID, nb: nb, output: out.OutputID, model: m}
}

func (f threadFixture) send(t *testing.T, text string) (ThreadReply, []notebook.Suggestion, error) {
	t.Helper()
	var seen []notebook.Suggestion
	r, err := f.s.SendThread(context.Background(), f.nbID, f.output, ChatSend{Text: text}, nil,
		func(sg notebook.Suggestion) { seen = append(seen, sg) })
	return r, seen, err
}

func call(name, args string) llm.ToolCall {
	return llm.ToolCall{ID: name + "-1", Name: name, Arguments: args}
}

func TestSendThreadUsesOutputAndMemory(t *testing.T) {
	f := newThreadFixture(t)
	st, _ := f.s.chatStore()
	if _, err := st.SetMemory([]string{"chat-only fact"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.send(t, "why?"); err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, m := range f.model.requests()[0].Messages {
		all.WriteString(m.Content + "\n")
	}
	text := all.String()
	for _, want := range []string{"Board digest", "board meets monthly", "Summarize the board.", "why?"} {
		if !strings.Contains(text, want) {
			t.Errorf("request lacks %q", want)
		}
	}
	if strings.Contains(text, "chat-only fact") {
		t.Error("global chat memory leaked into the thread")
	}
	sess, err := f.s.GetThread(f.nbID, f.output)
	if err != nil || len(sess.Messages) != 2 || sess.ID != f.output {
		t.Fatalf("thread: %v %+v", err, sess)
	}
}

func TestSendThreadStoresToolSuggestions(t *testing.T) {
	f := newThreadFixture(t,
		step{calls: []llm.ToolCall{call("suggest_memory", `{"text":"cite file paths","reason":"r"}`)}},
		step{text: "ok"})
	r, seen, err := f.send(t, "always cite files")
	if err != nil {
		t.Fatal(err)
	}
	if r.Assistant.Content != "ok" || len(r.Suggestions) != 1 || len(seen) != 1 {
		t.Fatalf("reply %+v seen %d", r, len(seen))
	}
	sg := r.Suggestions[0]
	if sg.Kind != notebook.SuggestMemory || sg.Text != "cite file paths" || sg.Status != notebook.SuggestOpen ||
		sg.MessageID != r.Assistant.ID {
		t.Fatalf("suggestion %+v", sg)
	}
	reqs := f.model.requests()
	if len(reqs) != 2 || len(reqs[0].Tools) != 2 || len(reqs[1].Tools) != 0 {
		t.Fatalf("requests: %d, tools %d then %d", len(reqs), len(reqs[0].Tools), len(reqs[1].Tools))
	}
	last := reqs[1].Messages[len(reqs[1].Messages)-1]
	if last.Role != llm.RoleTool || last.Content != "recorded" || last.ToolCallID != "suggest_memory-1" {
		t.Fatalf("tool result %+v", last)
	}
	stored, _ := f.nb.Suggestions(f.output)
	if len(stored) != 1 {
		t.Fatalf("stored %d", len(stored))
	}
}

func TestSendThreadPromptSuggestionHasBase(t *testing.T) {
	f := newThreadFixture(t,
		step{calls: []llm.ToolCall{call("suggest_prompt", `{"text":"Summarize the board. Cite files."}`)}})
	r, _, err := f.send(t, "cite files")
	if err != nil || len(r.Suggestions) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	if r.Suggestions[0].Base != promptDigest("Summarize the board.") {
		t.Fatalf("base %q", r.Suggestions[0].Base)
	}
}

func TestSendThreadDropsBadCalls(t *testing.T) {
	good := `{"text":"fact"}`
	f := newThreadFixture(t, step{calls: []llm.ToolCall{
		call("run_shell", good), call("suggest_memory", "not json"), call("suggest_memory", `{"text":"  "}`),
		call("suggest_memory", good), call("suggest_memory", good), call("suggest_memory", good),
		call("suggest_memory", good),
	}})
	r, _, err := f.send(t, "q")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Suggestions) != maxSuggestionsPerReply || r.Assistant.Content != "ok" {
		t.Fatalf("got %d suggestions, reply %q", len(r.Suggestions), r.Assistant.Content)
	}
}

func TestSendThreadToolsUnsupported(t *testing.T) {
	f := newThreadFixture(t, step{err: llm.ErrToolsUnsupported}, step{text: "plain"})
	r, _, err := f.send(t, "q")
	if err != nil || r.Assistant.Content != "plain" {
		t.Fatalf("%v %+v", err, r)
	}
	if reqs := f.model.requests(); len(reqs) != 2 || len(reqs[1].Tools) != 0 {
		t.Fatalf("retry without tools: %d", len(reqs))
	}
}

func TestSendThreadBusy(t *testing.T) {
	f := newThreadFixture(t)
	f.model.hold, f.model.release = make(chan struct{}), make(chan struct{})
	hold := f.model.hold
	done := make(chan error, 1)
	go func() { _, _, err := f.send(t, "first"); done <- err }()
	<-hold
	if _, _, err := f.send(t, "second"); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	close(f.model.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSendThreadMissingOutput(t *testing.T) {
	f := newThreadFixture(t)
	_, err := f.s.SendThread(context.Background(), f.nbID, notebook.NewID(), ChatSend{Text: "q"}, nil, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing output: %v", err)
	}
	if _, err := f.s.GetThread(f.nbID, "../x"); !IsInvalid(err) && !errors.Is(err, ErrNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}

func TestSendThreadRedactsSuggestion(t *testing.T) {
	f := newThreadFixture(t, step{calls: []llm.ToolCall{
		call("suggest_memory", `{"text":"use key k-secret-value-123 always"}`),
	}})
	f.s.Secrets = secrets.Map{"TESSIVO_LITELLM_KEY": "k-secret-value-123"}
	r, _, err := f.send(t, "q")
	if err != nil || len(r.Suggestions) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	if strings.Contains(r.Suggestions[0].Text, "k-secret-value-123") {
		t.Fatalf("secret stored: %q", r.Suggestions[0].Text)
	}
}

func TestThreadSettings(t *testing.T) {
	f := newThreadFixture(t)
	sess, err := f.s.SetThreadSettings(f.nbID, f.output, "litellm/other", "high")
	if err != nil || sess.Model != "litellm/other" || sess.ThinkingEffort != "high" {
		t.Fatalf("%v %+v", err, sess)
	}
	if _, err := f.s.SetThreadSettings(f.nbID, f.output, "", "extreme"); !IsInvalid(err) {
		t.Fatalf("bad effort: %v", err)
	}
	if err := f.s.CheckThreadSend(f.nbID, f.output, ChatSend{Text: ""}); !IsInvalid(err) {
		t.Fatalf("empty text: %v", err)
	}
}

func TestPromptTargets(t *testing.T) {
	f := newThreadFixture(t)
	run := notebook.Run{Prompt: "inline"}
	tg, err := f.s.promptTarget(f.nb, run)
	if err != nil || tg.Kind != targetNew || tg.Current != "inline" {
		t.Fatalf("new: %v %+v", err, tg)
	}
	if err := f.nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "saved text"}); err != nil {
		t.Fatal(err)
	}
	schedules := []notebook.Schedule{
		{ID: "a", Cron: "@daily", PromptName: "weekly", Missed: notebook.MissedSkip},
		{ID: "b", Cron: "@daily", PromptName: "weekly", Missed: notebook.MissedSkip},
		{ID: "c", Cron: "@daily", Prompt: "schedule text", Missed: notebook.MissedSkip},
	}
	if err := f.nb.SaveSchedules(schedules); err != nil {
		t.Fatal(err)
	}
	tg, err = f.s.promptTarget(f.nb, notebook.Run{PromptName: "weekly"})
	if err != nil || tg.Kind != targetSaved || tg.Current != "saved text" || len(tg.Schedules) != 2 {
		t.Fatalf("saved: %v %+v", err, tg)
	}
	tg, err = f.s.promptTarget(f.nb, notebook.Run{ScheduleID: "a", Prompt: "saved text"})
	if err != nil || tg.Kind != targetSaved || tg.PromptName != "weekly" {
		t.Fatalf("schedule named: %v %+v", err, tg)
	}
	tg, err = f.s.promptTarget(f.nb, notebook.Run{ScheduleID: "c", Prompt: "schedule text"})
	if err != nil || tg.Kind != targetSchedule || tg.ScheduleID != "c" || tg.Current != "schedule text" {
		t.Fatalf("schedule inline: %v %+v", err, tg)
	}
}

func TestRunRecordsPromptNameAndSchedule(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Board", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	_ = nb.SavePrompt(notebook.Prompt{Name: "weekly", Text: "Summarize."})
	out, err := s.Run(context.Background(),
		RunRequest{Notebook: nb, PromptName: "weekly", ScheduleID: "s1", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	run, err := nb.LoadRun(out.RunID)
	if err != nil || run.PromptName != "weekly" || run.ScheduleID != "s1" {
		t.Fatalf("%v %+v", err, run)
	}
}

func TestThreadNotesAreKept(t *testing.T) {
	f := newThreadFixture(t)
	st, err := f.s.threadStore(f.nb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Ensure(f.output, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Update(f.output, func(s *chat.Session) error {
		s.Messages = append(s.Messages, chat.Message{ID: chat.NewMessageID(), Role: chat.RoleNote, Content: "n"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.send(t, "q"); err != nil {
		t.Fatal(err)
	}
	for _, m := range f.model.requests()[0].Messages {
		if m.Content == "n" {
			t.Fatal("note sent to the model")
		}
	}
}

func TestThreadNotebookByListedFolder(t *testing.T) {
	f := newThreadFixture(t)
	if _, err := f.s.GetThread(f.nb.Dir, f.output); err == nil {
		t.Fatal("an unlisted folder must not be opened")
	}
	f.s.NotebookDirs = []string{f.nb.Dir}
	if _, err := f.s.GetThread(f.nb.Dir, f.output); err != nil {
		t.Fatalf("listed folder: %v", err)
	}
	sg := f.addSuggestion(t, notebook.SuggestMemory, "from the cli")
	if _, err := f.s.ApplySuggestion(f.nb.Dir, f.output, sg.ID); err != nil {
		t.Fatalf("apply through a folder: %v", err)
	}
	if mem := f.reopen(t).Manifest.Memory; mem[len(mem)-1] != "from the cli" {
		t.Fatalf("memory %v", mem)
	}
}
