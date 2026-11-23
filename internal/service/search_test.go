package service

import (
	"context"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

func TestSearchFindsNotebooksOutputsPromptsAndNotes(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Payments risk", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	if _, err := s.Run(context.Background(), RunRequest{Notebook: nb, Prompt: "x", Save: true}, nil); err != nil {
		t.Fatal(err)
	}
	weekly := notebook.Prompt{Name: "weekly", Text: "Summarize the weekly incidents"}
	if _, err := s.SavePrompt(info.ID, weekly); err != nil {
		t.Fatal(err)
	}
	r, _ := s.CreateResearch("Vendor study", "")
	audit := research.Note{Title: "Audit", Text: "The vendor failed the SOC 2 audit"}
	if _, err := s.SaveResearchNote(r.ID, audit); err != nil {
		t.Fatal(err)
	}
	for q, kind := range map[string]string{
		"payments":        SearchNotebook,
		"board digest":    SearchOutput, // the summary output's title
		"weekly incident": SearchPrompt,
		"soc 2 audit":     SearchNote,
		"vendor study":    SearchResearch,
	} {
		res, err := s.Search(q, 20)
		if err != nil || len(res) == 0 || res[0].Kind != kind {
			t.Errorf("%q: %v %+v", q, err, res)
			continue
		}
		if res[0].Snippet == "" || res[0].Title == "" {
			t.Errorf("%q: no title or snippet: %+v", q, res[0])
		}
	}
	if res, _ := s.Search("nothing matches this", 20); len(res) != 0 {
		t.Fatalf("unexpected: %+v", res)
	}
	if _, err := s.Search("  ", 20); !IsInvalid(err) {
		t.Fatalf("blank query: %v", err)
	}
	if res, _ := s.Search("a", 1); len(res) > 1 {
		t.Fatalf("limit ignored: %d", len(res))
	}
	long := strings.Repeat("x ", 200) + "needle here" + strings.Repeat(" y", 200)
	if got := snippet(long, "needle"); !strings.Contains(got, "needle") || len(got) > 200 {
		t.Fatalf("snippet %q", got)
	}
}
