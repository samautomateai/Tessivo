package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func suggestion(t *testing.T, s *Service, workType string, files map[string]string) WorkTypeSuggestion {
	t.Helper()
	info, err := s.CreateNotebook("N"+workType+filepath.Base(t.TempDir()), workType)
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		p := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if body == "/" { // a folder
			if err := os.MkdirAll(p, 0o700); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AddContext(info.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SuggestWorkType(info.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSuggestWorkType(t *testing.T) {
	s := newSvc(t, summary)
	code := t.TempDir()
	_ = os.WriteFile(filepath.Join(code, "go.mod"), []byte("module x"), 0o600)
	info, _ := s.CreateNotebook("Code", "summarize")
	if _, err := s.AddContext(info.ID, code); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.SuggestWorkType(info.ID); got.WorkType != "analyze" || got.Reason == "" {
		t.Fatalf("a code base: %+v", got)
	}
	cases := []struct {
		workType string
		files    map[string]string
		want     string
	}{
		{"summarize", map[string]string{"fix.patch": "diff --git a b"}, "analyze"},
		{"analyze", map[string]string{"q3.csv": "a,b\n1,2", "q2.csv": "a,b\n1,2", "readme.txt": "x"}, "report"},
		{"report", map[string]string{"standup.vtt": "WEBVTT"}, "summarize"},
		{"analyze", map[string]string{"meeting-minutes.md": "# Minutes"}, "summarize"},
		{"summarize", map[string]string{"docs/guide.md": "# Guide"}, ""}, // a file, not a docs folder
		{"analyze", map[string]string{"q3.csv": "a,b"}, "report"},
		{"report", map[string]string{"q3.csv": "a,b"}, ""}, // already the suggestion
		{"summarize", map[string]string{}, ""},             // no sources
	}
	for _, c := range cases {
		if got := suggestion(t, s, c.workType, c.files); got.WorkType != c.want {
			t.Errorf("%s %v: got %+v, want %q", c.workType, c.files, got, c.want)
		}
	}
	docs := filepath.Join(t.TempDir(), "docs")
	_ = os.MkdirAll(docs, 0o700)
	_ = os.WriteFile(filepath.Join(docs, "a.md"), []byte("# A"), 0o600)
	d, _ := s.CreateNotebook("Docs", "summarize")
	_, _ = s.AddContext(d.ID, docs)
	if got, _ := s.SuggestWorkType(d.ID); got.WorkType != "document" {
		t.Fatalf("a docs folder: %+v", got)
	}
}

func TestWorkTypeChangesOnlyWithoutOutputs(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Board", "analyze")
	p := filepath.Join(t.TempDir(), "standup.vtt")
	_ = os.WriteFile(p, []byte("WEBVTT"), 0o600)
	_, _ = s.AddContext(info.ID, p)
	wt := "summarize"
	got, err := s.UpdateNotebook(info.ID, NotebookUpdate{WorkType: &wt})
	if err != nil || got.WorkType != "summarize" {
		t.Fatalf("switch: %v %+v", err, got)
	}
	bad := "poem"
	if _, err := s.UpdateNotebook(info.ID, NotebookUpdate{WorkType: &bad}); !IsInvalid(err) {
		t.Fatalf("unknown work type: %v", err)
	}
	nb := mustOpen(t, s, info.ID)
	if _, err := s.Run(context.Background(), RunRequest{Notebook: nb, Save: true}, nil); err != nil {
		t.Fatal(err)
	}
	if sug, _ := s.SuggestWorkType(info.ID); sug.WorkType != "" {
		t.Fatalf("no suggestion once there are outputs: %+v", sug)
	}
	back := "analyze"
	if _, err := s.UpdateNotebook(info.ID, NotebookUpdate{WorkType: &back}); !errors.Is(err, ErrConflict) {
		t.Fatalf("after an output: %v", err)
	}
}
