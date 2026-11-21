package chat

import (
	"strings"
	"testing"
)

func TestSuggestWorkType(t *testing.T) {
	cases := map[string]string{
		"Assess the risk and impact of this change":    "analyze",
		"Give me a quick TL;DR digest of these":        "summarize",
		"Explain how the architecture fits together":   "document",
		"Write the weekly status report with the KPIs": "report",
		"hello there": "summarize",
		"Summarize the risk: summary first, then summary": "summarize",
	}
	for text, want := range cases {
		s := Session{Messages: []Message{
			{Role: RoleUser, Content: text},
			{Role: RoleAssistant, Content: "report report"},
		}}
		if got := SuggestWorkType(s); got != want {
			t.Errorf("%q: got %s want %s", text, got, want)
		}
	}
}

func TestSessionHelpers(t *testing.T) {
	a := Attachment{Kind: AttachPath, Ref: "/a"}
	s := Session{Messages: []Message{
		{ID: "u1", Role: RoleUser, Content: "first", Attachments: []Attachment{a}},
		{ID: "a1", Role: RoleAssistant, Content: "one"},
		{ID: "u2", Role: RoleUser, Content: "second", Attachments: []Attachment{a, {Kind: AttachNotebook, Ref: "nb"}}},
		{ID: "a2", Role: RoleAssistant, Content: "two"},
	}}
	if LastUserMessage(s) != "second" || LastUserMessage(Session{}) != "" {
		t.Fatal("last user message")
	}
	if UserText(s) != "first\nsecond\n" {
		t.Fatalf("user text %q", UserText(s))
	}
	if m, ok := s.Find("a2"); !ok || m.Content != "two" {
		t.Fatal("find")
	}
	if _, ok := s.Find("zz"); ok {
		t.Fatal("found a missing message")
	}
	if s.QuestionBefore("a2") != "second" || s.QuestionBefore("u1") != "" || s.QuestionBefore("zz") != "" {
		t.Fatal("question before")
	}
	if got := s.Attachments(); len(got) != 2 || got[0] != a {
		t.Fatalf("attachments %+v", got)
	}
	if w := Words("Hello, Wörld! 42 x"); strings.Join(w, " ") != "hello wörld 42 x" {
		t.Fatalf("words %v", w)
	}
	if n := len(Words(strings.Repeat("w ", 6000))); n != 5000 {
		t.Fatalf("words not capped: %d", n)
	}
}
