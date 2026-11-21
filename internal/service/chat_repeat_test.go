package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/template"
)

// chatWith creates a chat and sends the given messages (each with optional attachments).
func chatWith(t *testing.T, s *Service, msgs ...ChatSend) chat.Session {
	t.Helper()
	sess, err := s.CreateChat(ChatInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range msgs {
		if _, err := s.SendChat(context.Background(), sess.ID, m, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.GetChat(sess.ID)
	return got
}

func TestPlanAndMakeRepeatable(t *testing.T) {
	s := newSvc(t, summary)
	s.Templates = template.Library{Dir: filepath.Join(t.TempDir(), "templates")}
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.md")
	_ = os.WriteFile(notes, []byte("Decided to ship on Friday."), 0o600)
	src := filepath.Join(dir, "minutes.md")
	_ = os.WriteFile(src, []byte("minutes"), 0o600)
	nb, _ := s.CreateNotebook("Board", "summarize")
	if _, err := s.AddContext(nb.ID, src); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Run(context.Background(), RunRequest{Notebook: mustOpen(t, s, nb.ID), Save: true}, nil); err != nil {
		t.Fatal(err)
	}
	outs, _ := s.ListOutputs(nb.ID, "")
	sess := chatWith(t, s,
		ChatSend{Text: "Summarize these meeting notes", Attachments: []chat.Attachment{
			{Kind: chat.AttachPath, Ref: notes},
			{Kind: chat.AttachNotebook, Ref: nb.ID},
			{Kind: chat.AttachOutput, Ref: nb.ID + "/" + outs[0].ID, Label: "Board digest"},
		}},
		ChatSend{Text: "Now list the action items with owners"},
	)
	plan, err := s.PlanRepeatable(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.WorkType != "summarize" || plan.TemplateID != "meeting-notes-to-actions" ||
		plan.Prompt != "Now list the action items with owners" || plan.Name != "Summarize these meeting notes" {
		t.Fatalf("plan %+v", plan)
	}
	if len(plan.Context) != 2 || plan.Context[0] != notes || len(plan.Skipped) != 1 ||
		!strings.HasPrefix(plan.Skipped[0], "Board digest:") {
		t.Fatalf("plan context %+v skipped %+v", plan.Context, plan.Skipped)
	}

	info, err := s.MakeRepeatable(sess.ID, RepeatableInput{Name: "Weekly actions"})
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "weekly-actions" || !strings.HasPrefix(info.Template, "meeting-notes-to-actions@") ||
		len(info.Context) != 2 || info.Context[1].Label != "minutes.md" {
		t.Fatalf("notebook %+v", info)
	}
	p, err := mustOpen(t, s, info.ID).LoadPrompt(ChatPromptName)
	if err != nil || p.Text != plan.Prompt {
		t.Fatalf("prompt %v %+v", err, p)
	}

	plain, err := s.MakeRepeatable(sess.ID, RepeatableInput{WorkType: "report", TemplateID: "none", Prompt: "Weekly"})
	if err != nil || plain.WorkType != "report" || plain.Template != "" {
		t.Fatalf("override %v %+v", err, plain)
	}
}

func TestRepeatableErrors(t *testing.T) {
	s := newSvc(t, summary)
	empty, _ := s.CreateChat(ChatInput{})
	if _, err := s.PlanRepeatable(empty.ID); !IsInvalid(err) {
		t.Fatalf("empty chat: %v", err)
	}
	if _, err := s.PlanRepeatable("ffffffffffffffff"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing chat: %v", err)
	}
	gone := filepath.Join(t.TempDir(), "gone.md")
	_ = os.WriteFile(gone, []byte("x"), 0o600)
	sess := chatWith(t, s, ChatSend{
		Text:        "explain how the architecture works",
		Attachments: []chat.Attachment{{Kind: chat.AttachPath, Ref: gone}},
	})
	if plan, _ := s.PlanRepeatable(sess.ID); plan.WorkType != "document" {
		t.Fatalf("work type %+v", plan)
	}
	cases := map[string]RepeatableInput{
		"bad type":            {WorkType: "poetry"},
		"unknown template":    {TemplateID: "nope"},
		"template type clash": {WorkType: "report", TemplateID: "contract-digest"},
	}
	for name, in := range cases {
		if _, err := s.MakeRepeatable(sess.ID, in); !IsInvalid(err) && !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	_ = os.Remove(gone)
	if _, err := s.MakeRepeatable(sess.ID, RepeatableInput{}); !IsInvalid(err) {
		t.Fatalf("vanished context: %v", err)
	}
}

func TestSaveChatReply(t *testing.T) {
	s := newSvc(t, summary)
	sess := chatWith(t, s, ChatSend{Text: "Summarize the board pack"})
	reply := sess.Messages[1]
	out, err := s.SaveChatReply(context.Background(), sess.ID, reply.ID, "summarize")
	if err != nil {
		t.Fatal(err)
	}
	if out.NotebookID != "chat-summarize" || out.OutputID == "" || out.Title != "Board digest" ||
		!strings.Contains(out.Markdown, "Board digest") {
		t.Fatalf("output %+v", out)
	}
	if o, err := s.GetOutput("chat-summarize", out.OutputID); err != nil || o.Title != "Board digest" {
		t.Fatalf("saved output: %v %+v", err, o)
	}
	again, err := s.SaveChatReply(context.Background(), sess.ID, reply.ID, "summarize")
	if err != nil || again.NotebookID != "chat-summarize" {
		t.Fatalf("second save reuses the notebook: %v %+v", err, again)
	}

	if _, err := s.SaveChatReply(context.Background(), sess.ID, sess.Messages[0].ID, "summarize"); !IsInvalid(err) {
		t.Fatalf("user message: %v", err)
	}
	if _, err := s.SaveChatReply(context.Background(), sess.ID, "nope", "summarize"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing message: %v", err)
	}
	if _, err := s.SaveChatReply(context.Background(), sess.ID, reply.ID, "poetry"); !IsInvalid(err) {
		t.Fatalf("bad type: %v", err)
	}
	_, err = s.SaveChatReply(context.Background(), "ffffffffffffffff", reply.ID, "summarize")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing chat: %v", err)
	}
	if _, err := s.CreateNotebook("Chat analyze", "summarize"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveChatReply(context.Background(), sess.ID, reply.ID, "analyze"); !IsInvalid(err) {
		t.Fatalf("clashing notebook: %v", err)
	}
}

func TestSaveChatReplyInvalidOutput(t *testing.T) {
	s := newSvc(t, "not json at all")
	sess := chatWith(t, s, ChatSend{Text: "hi"})
	_, err := s.SaveChatReply(context.Background(), sess.ID, sess.Messages[1].ID, "summarize")
	if !errors.Is(err, llm.ErrInvalidOutput) {
		t.Fatalf("want invalid output, got %v", err)
	}
}

func mustOpen(t *testing.T, s *Service, id string) *notebook.Notebook {
	t.Helper()
	nb, err := s.OpenNotebook(id)
	if err != nil {
		t.Fatal(err)
	}
	return nb
}
