package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
)

func TestChatWebPageAttachment(t *testing.T) {
	fetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fetches++
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body><h1>Status</h1><p>All systems normal.</p></body></html>"))
	}))
	defer srv.Close()
	m := &chatModel{pieces: []string{"Normal."}}
	s := chatSvc(t, m)
	sess, _ := s.CreateChat(ChatInput{})
	page := srv.URL + "/status/"
	in := ChatSend{Text: "Anything down?", Attachments: []chat.Attachment{{Kind: chat.AttachURL, Ref: page}}}
	reply, err := s.SendChat(context.Background(), sess.ID, in, nil)
	if err != nil {
		t.Fatal(err)
	}
	att := reply.User.Attachments[0]
	if !strings.HasSuffix(att.Label, "/status") || att.Tokens == 0 {
		t.Fatalf("attachment %+v", att)
	}
	if last := m.reqs[0].Messages[len(m.reqs[0].Messages)-1].Content; !strings.Contains(last, "All systems normal") {
		t.Fatalf("the page text must reach the model: %q", last)
	}
	if _, err := s.SendChat(context.Background(), sess.ID, ChatSend{Text: "And now?"}, nil); err != nil {
		t.Fatal(err)
	}
	if fetches != 2 {
		t.Fatalf("the page is fetched again for every turn: %d fetches", fetches)
	}

	plan, err := s.PlanRepeatable(sess.ID)
	if err != nil || len(plan.Context) != 1 || plan.Context[0] != page {
		t.Fatalf("plan: %v %+v", err, plan)
	}
	nb, err := s.MakeRepeatable(sess.ID, RepeatableInput{WorkType: "summarize"})
	if err != nil || len(nb.Context) != 1 || nb.Context[0].Kind != "url" || nb.Context[0].URI != page {
		t.Fatalf("the page becomes a web page source: %v %+v", err, nb.Context)
	}

	for name, a := range map[string]chat.Attachment{
		"file scheme": {Kind: chat.AttachURL, Ref: "file:///etc/passwd"},
		"credentials": {Kind: chat.AttachURL, Ref: "https://user:pw@example.com/"},
		"unreachable": {Kind: chat.AttachURL, Ref: "http://127.0.0.1:1/"},
	} {
		in := ChatSend{Text: "x", Attachments: []chat.Attachment{a}}
		if _, err := s.SendChat(context.Background(), sess.ID, in, nil); !IsInvalid(err) {
			t.Errorf("%s: want invalid, got %v", name, err)
		}
	}
}
