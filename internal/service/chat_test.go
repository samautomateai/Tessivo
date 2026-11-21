package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// chatModel records requests and replies in pieces, optionally failing or
// waiting for cancellation after the first piece.
type chatModel struct {
	mu      sync.Mutex
	reqs    []llm.Request
	pieces  []string
	fail    error
	block   bool
	started chan struct{}
}

func (m *chatModel) ChatStream(ctx context.Context, req llm.Request, onDelta func(string)) (llm.Result, error) {
	m.mu.Lock()
	m.reqs = append(m.reqs, req)
	m.mu.Unlock()
	var sb strings.Builder
	for i, p := range m.pieces {
		onDelta(p)
		sb.WriteString(p)
		if i == 0 && m.block {
			close(m.started)
			<-ctx.Done()
			return llm.Result{Response: llm.Response{Content: sb.String()}, Target: "litellm/m"}, ctx.Err()
		}
	}
	if m.fail != nil {
		return llm.Result{}, m.fail
	}
	res := llm.Result{Response: llm.Response{Content: sb.String()}, Target: "litellm/m"}
	res.Usage.CompletionTokens = 7
	return res, nil
}

func chatSvc(t *testing.T, m *chatModel) *Service {
	t.Helper()
	s := newSvc(t, "")
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) { return m, nil }
	return s
}

func TestChatSessionsThroughService(t *testing.T) {
	s := chatSvc(t, &chatModel{})
	sess, err := s.CreateChat(ChatInput{Title: "Ops", Model: "litellm/m", ThinkingEffort: "low"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.Workspace), ChatDirName, "sessions")); err != nil {
		t.Fatalf("store not beside workspace: %v", err)
	}
	title, pinned := "Ops review", true
	upd, err := s.UpdateChat(sess.ID, ChatPatch{Title: &title, Pinned: &pinned})
	if err != nil || upd.Title != title || !upd.Pinned || upd.Model != "litellm/m" {
		t.Fatalf("update: %v %+v", err, upd)
	}
	archived, model, effort := true, "", "high"
	upd, _ = s.UpdateChat(sess.ID, ChatPatch{Archived: &archived, Model: &model, ThinkingEffort: &effort})
	if !upd.Archived || upd.Model != "" || upd.ThinkingEffort != "high" {
		t.Fatalf("archive: %+v", upd)
	}
	if list, _ := s.ListChats("", false); len(list) != 0 {
		t.Fatalf("archived listed as active: %+v", list)
	}
	if list, _ := s.ListChats("review", true); len(list) != 1 {
		t.Fatalf("archived search: %+v", list)
	}
	if got, err := s.GetChat(sess.ID); err != nil || got.ID != sess.ID {
		t.Fatalf("get: %v", err)
	}
	if err := s.DeleteChat(sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChat(sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if err := s.DeleteChat(sess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestChatValidation(t *testing.T) {
	s := chatSvc(t, &chatModel{})
	bad := ChatInput{Model: "nope"}
	if _, err := s.CreateChat(bad); !IsInvalid(err) {
		t.Fatalf("unknown model: %v", err)
	}
	if _, err := s.CreateChat(ChatInput{ThinkingEffort: "max"}); !IsInvalid(err) {
		t.Fatalf("bad effort: %v", err)
	}
	if _, err := s.CreateChat(ChatInput{Title: "a\nb"}); !IsInvalid(err) {
		t.Fatalf("bad title: %v", err)
	}
	sess, _ := s.CreateChat(ChatInput{})
	long := strings.Repeat("t", chat.MaxTitle+1)
	if _, err := s.UpdateChat(sess.ID, ChatPatch{Title: &long}); !IsInvalid(err) {
		t.Fatalf("long title: %v", err)
	}
	m := "nope"
	if _, err := s.UpdateChat(sess.ID, ChatPatch{Model: &m}); !IsInvalid(err) {
		t.Fatalf("patch model: %v", err)
	}
	if _, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: " "}, nil); !IsInvalid(err) {
		t.Fatalf("empty message: %v", err)
	}
	_, err := s.SendChat(context.Background(), "ffffffffffffffff", ChatSend{Text: "hi"}, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing session: %v", err)
	}
	if _, err := s.ListChats(strings.Repeat("q", chat.MaxQuery+1), false); !IsInvalid(err) {
		t.Fatalf("long query: %v", err)
	}
	noWS := &Service{}
	if _, err := noWS.ListChats("", false); err == nil {
		t.Fatal("no workspace accepted")
	}
	s.Config.Defaults.Model = ""
	if _, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "hi"}, nil); !IsInvalid(err) {
		t.Fatalf("no model: %v", err)
	}
}

func TestChatMemoryThroughService(t *testing.T) {
	s := chatSvc(t, &chatModel{})
	if m, err := s.ChatMemory(); err != nil || len(m) != 0 {
		t.Fatalf("%v %v", m, err)
	}
	if m, err := s.SetChatMemory([]string{"I'm on call this week", " "}); err != nil || len(m) != 1 {
		t.Fatalf("%v %v", m, err)
	}
	if _, err := s.SetChatMemory(make([]string, chat.MaxMemory+1)); !IsInvalid(err) {
		t.Fatalf("limit: %v", err)
	}
}

func TestSendChatStreamsStoresAndAudits(t *testing.T) {
	m := &chatModel{pieces: []string{"Rotate ", "monthly."}}
	s := chatSvc(t, m)
	_, _ = s.SetChatMemory([]string{"I lead payments"})
	dir := t.TempDir()
	doc := filepath.Join(dir, "runbook.md")
	_ = os.WriteFile(doc, []byte("# Key rotation runbook"), 0o600)
	sess, _ := s.CreateChat(ChatInput{ThinkingEffort: "medium"})

	var streamed strings.Builder
	in := ChatSend{
		Text:        "How often do we rotate keys?",
		Attachments: []chat.Attachment{{Kind: chat.AttachPath, Ref: doc}},
	}
	reply, err := s.SendChat(context.Background(), sess.ID, in, func(d string) { streamed.WriteString(d) })
	if err != nil {
		t.Fatal(err)
	}
	want := "Rotate monthly."
	if streamed.String() != want || reply.Assistant.Content != want || reply.Assistant.Model != "litellm/m" {
		t.Fatalf("reply %+v streamed %q", reply, streamed.String())
	}
	att := reply.User.Attachments[0]
	if att.Label != "runbook.md" || att.Tokens == 0 {
		t.Fatalf("attachment %+v", att)
	}
	req := m.reqs[0]
	if req.ReasoningEffort != "medium" || req.Model != "litellm/m" {
		t.Fatalf("request settings %+v", req)
	}
	if !strings.Contains(req.Messages[0].Content, "I lead payments") ||
		!strings.Contains(req.Messages[len(req.Messages)-1].Content, "Key rotation runbook") {
		t.Fatalf("memory or context missing: %+v", req.Messages)
	}
	got, _ := s.GetChat(sess.ID)
	if got.Title != "How often do we rotate keys?" || len(got.Messages) != 2 {
		t.Fatalf("stored %+v", got)
	}

	// A follow-up still sees context attached earlier, plus the history.
	if _, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "And who approves?"}, nil); err != nil {
		t.Fatal(err)
	}
	follow := m.reqs[1].Messages
	if len(follow) != 4 || !strings.Contains(follow[3].Content, "Key rotation runbook") {
		t.Fatalf("follow-up %+v", follow)
	}
	n, err := audit.VerifyFile(s.UserAuditPath)
	if err != nil || n != 2 {
		t.Fatalf("audit: %d %v", n, err)
	}
	b, _ := os.ReadFile(s.UserAuditPath)
	if strings.Contains(string(b), "rotate keys") {
		t.Fatal("audit log holds the prompt text")
	}
}

func TestSendChatFailureIsStored(t *testing.T) {
	m := &chatModel{pieces: []string{"Part"}, fail: &llm.APIError{Status: 429, Body: "slow down"}}
	s := chatSvc(t, m)
	sess, _ := s.CreateChat(ChatInput{})
	reply, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "hi"}, nil)
	if err == nil || !reply.Assistant.Stopped || !strings.HasPrefix(reply.Assistant.Error, "model error") {
		t.Fatalf("%v %+v", err, reply)
	}
	if reply.Assistant.Content != "Part" {
		t.Fatalf("partial text lost: %q", reply.Assistant.Content)
	}
	if got, _ := s.GetChat(sess.ID); len(got.Messages) != 2 {
		t.Fatalf("failed turn not stored: %+v", got.Messages)
	}
}

func TestSendChatStopKeepsPartialAndBlocksOverlap(t *testing.T) {
	m := &chatModel{pieces: []string{"Half an ans", "wer"}, block: true, started: make(chan struct{})}
	s := chatSvc(t, m)
	sess, _ := s.CreateChat(ChatInput{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan ChatReply)
	go func() {
		r, err := s.SendChat(ctx, sess.ID, ChatSend{Text: "explain"}, nil)
		if err != nil {
			t.Error(err)
		}
		done <- r
	}()
	<-m.started
	if _, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "again"}, nil); !errors.Is(err, ErrConflict) {
		t.Errorf("overlapping send: %v", err)
	}
	cancel()
	r := <-done
	if !r.Assistant.Stopped || r.Assistant.Error != "" || r.Assistant.Content != "Half an ans" {
		t.Fatalf("stopped reply %+v", r.Assistant)
	}
}

func TestChatAttachments(t *testing.T) {
	s := chatSvc(t, &chatModel{pieces: []string{"ok"}})
	nb, _ := s.CreateNotebook("Board", "summarize")
	src := filepath.Join(t.TempDir(), "minutes.md")
	_ = os.WriteFile(src, []byte("minutes text"), 0o600)
	if _, err := s.AddContext(nb.ID, src); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.CreateChat(ChatInput{})
	in := ChatSend{Text: "summarize", Attachments: []chat.Attachment{{Kind: chat.AttachNotebook, Ref: nb.ID}}}
	r, err := s.SendChat(context.Background(), sess.ID, in, nil)
	if err != nil || r.User.Attachments[0].Label != "Board" {
		t.Fatalf("notebook attachment: %v %+v", err, r.User)
	}
	cases := map[string]chat.Attachment{
		"relative":     {Kind: chat.AttachPath, Ref: "minutes.md"},
		"missing file": {Kind: chat.AttachPath, Ref: filepath.Join(t.TempDir(), "gone.md")},
		"bad output":   {Kind: chat.AttachOutput, Ref: "no-slash"},
	}
	for name, a := range cases {
		_, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "x", Attachments: []chat.Attachment{a}}, nil)
		if !IsInvalid(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	missing := []chat.Attachment{{Kind: chat.AttachOutput, Ref: nb.ID + "/nope"}}
	_, err = s.SendChat(context.Background(), sess.ID, ChatSend{Text: "x", Attachments: missing}, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing output: %v", err)
	}
}

func TestChatModelSetupFailure(t *testing.T) {
	s := newSvc(t, "")
	s.NewModel = func(config.File, config.Alias, secrets.Resolver) (executor.Model, error) {
		return nil, errors.New("no key")
	}
	sess, _ := s.CreateChat(ChatInput{})
	r, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "hi"}, nil)
	if err == nil || r.Assistant.Error != "model setup failed" {
		t.Fatalf("%v %+v", err, r)
	}
}
