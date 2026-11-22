package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/research"
)

// researchWithSource is a notebook with one file source, already summarized.
func researchWithSource(t *testing.T) (s *Service, m *counting, nbID, srcID string) {
	t.Helper()
	s, m = researchSvc(t)
	nb, _ := s.CreateResearch("Board", "Board papers")
	info, err := s.AddResearchSource(context.Background(), nb.ID, writeSource(t, "minutes.md", "FULL: ship in March"),
		"", "", true)
	if err != nil {
		t.Fatal(err)
	}
	return s, m, nb.ID, info.Sources[0].ID
}

func lastUser(m *counting) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	msgs := m.reqs[len(m.reqs)-1].Messages
	return msgs[len(msgs)-1].Content
}

func allText(m *counting) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sb strings.Builder
	for _, msg := range m.reqs[len(m.reqs)-1].Messages {
		sb.WriteString(msg.Content + "\n")
	}
	return sb.String()
}

func TestSendResearchCites(t *testing.T) {
	s, m, nbID, srcID := researchWithSource(t)
	chat, err := s.CreateResearchChat(nbID, "Plans")
	if err != nil {
		t.Fatal(err)
	}
	m.reply = "Ship in March [1]."
	r, err := s.SendResearch(context.Background(), nbID, chat.ID, "When do we ship?", nil)
	if err != nil || r.Assistant.Content != "Ship in March [1]." {
		t.Fatalf("%v %+v", err, r.Assistant)
	}
	if len(r.Citations) != 1 || r.Citations[0].ID != srcID || r.Citations[0].Label != "minutes.md" {
		t.Fatalf("citations: %+v", r.Citations)
	}
	_, cites, err := s.GetResearchChat(nbID, chat.ID)
	if err != nil || len(cites[r.Assistant.ID]) != 1 {
		t.Fatalf("sidecar: %v %+v", err, cites)
	}
}

func TestSendResearchUsesLevels(t *testing.T) {
	s, m, nbID, _ := researchWithSource(t)
	secret := writeSource(t, "secret.md", "OFF SOURCE TEXT")
	off, _ := s.AddResearchSource(context.Background(), nbID, secret, "", "", true)
	if _, err := s.SetSourceLevel(nbID, off.Sources[1].ID, research.LevelOff); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveResearchNote(nbID, research.Note{Title: "x", Text: "EXCLUDED NOTE"}); err != nil {
		t.Fatal(err)
	}
	chat, _ := s.CreateResearchChat(nbID, "")
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "q", nil); err != nil {
		t.Fatal(err)
	}
	user := lastUser(m)
	if !strings.Contains(user, m.reply) || strings.Contains(user, "FULL: ship in March") {
		t.Fatalf("summary level must send the summary, not the full text:\n%s", user)
	}
	if strings.Contains(user, "OFF SOURCE TEXT") || strings.Contains(user, "EXCLUDED NOTE") {
		t.Fatalf("off source or excluded note sent:\n%s", user)
	}
}

func TestHistoryRelabelled(t *testing.T) {
	s, m, nbID, _ := researchWithSource(t)
	chat, _ := s.CreateResearchChat(nbID, "")
	m.reply = "March [1]."
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "first?", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "second?", nil); err != nil {
		t.Fatal(err)
	}
	text := allText(m)
	if !strings.Contains(text, "March [minutes.md].") || strings.Contains(text, "March [1].") {
		t.Fatalf("history not relabelled:\n%s", text)
	}
}

func TestUnreadableSourceSkipped(t *testing.T) {
	s, _, nbID, _ := researchWithSource(t)
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o600)
	info, _ := s.AddResearchSource(context.Background(), nbID, dir, "", "", true)
	if _, err := s.SetSourceLevel(nbID, info.Sources[1].ID, research.LevelFull); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	chat, _ := s.CreateResearchChat(nbID, "")
	r, err := s.SendResearch(context.Background(), nbID, chat.ID, "q", nil)
	if err != nil || len(r.Skipped) != 1 || !strings.HasPrefix(r.Assistant.Content, "_Skipped") {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestResearchBusy409(t *testing.T) {
	s, m, nbID, _ := researchWithSource(t)
	chat, _ := s.CreateResearchChat(nbID, "")
	m.gate = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := s.SendResearch(context.Background(), nbID, chat.ID, "first", nil)
		done <- err
	}()
	for s.researchBusy(nbID, chat.ID) == false {
		select {
		case err := <-done:
			t.Fatalf("finished early: %v", err)
		default:
		}
	}
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "second", nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	m.gate <- struct{}{}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestNoteFromMessage(t *testing.T) {
	s, m, nbID, _ := researchWithSource(t)
	chat, _ := s.CreateResearchChat(nbID, "")
	m.reply = "Ship in March [1]."
	r, _ := s.SendResearch(context.Background(), nbID, chat.ID, strings.Repeat("When do we ship? ", 10), nil)
	n, err := s.NoteFromMessage(nbID, chat.ID, r.Assistant.ID)
	if err != nil || n.Origin != research.OriginAI || !n.Include || len([]rune(n.Title)) > 80 {
		t.Fatalf("%v %+v", err, n)
	}
	if !strings.Contains(n.Text, "Ship in March [minutes.md].") || !strings.Contains(n.Text, "References") {
		t.Fatalf("note text: %q", n.Text)
	}
}

func TestResearchChatModelPrecedence(t *testing.T) {
	s, m, nbID, _ := researchWithSource(t)
	model := "litellm/notebook"
	if _, err := s.UpdateResearch(nbID, nil, nil, &s.Config.Defaults); err != nil {
		t.Fatal(err)
	}
	settings := s.Config.Defaults
	settings.Model = model
	if _, err := s.UpdateResearch(nbID, nil, nil, &settings); err != nil {
		t.Fatal(err)
	}
	chat, _ := s.CreateResearchChat(nbID, "")
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "q", nil); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	got := m.reqs[len(m.reqs)-1].Model
	m.mu.Unlock()
	if got != model {
		t.Fatalf("notebook model not used: %q", got)
	}
	own := "litellm/chat"
	if _, err := s.UpdateResearchChat(nbID, chat.ID, ChatPatch{Model: &own}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendResearch(context.Background(), nbID, chat.ID, "q2", nil); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	got = m.reqs[len(m.reqs)-1].Model
	m.mu.Unlock()
	if got != own {
		t.Fatalf("conversation model not used: %q", got)
	}
}
