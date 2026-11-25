package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/research"
)

// exportedNotebook makes a notebook with a copied file, an in-place folder,
// a schedule, a run record, an output and an audit log, and exports it.
func exportedNotebook(t *testing.T, s *Service, outputs bool) (NotebookInfo, []byte) {
	t.Helper()
	info, _ := s.CreateNotebook("Ops", "summarize")
	file := filepath.Join(t.TempDir(), "notes.md")
	_ = os.WriteFile(file, []byte("ship in March"), 0o600)
	folder := t.TempDir()
	_ = os.WriteFile(filepath.Join(folder, "a.go"), []byte("package a"), 0o600)
	for _, p := range []string{file, folder} {
		if _, err := s.AddContext(info.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	on := true
	weekly := ScheduleInput{Cron: "0 8 * * MON", Prompt: "weekly", Enabled: &on}
	if _, err := s.AddSchedule(info.ID, weekly); err != nil {
		t.Fatal(err)
	}
	nb, _ := s.OpenNotebook(info.ID)
	_ = nb.SaveRun(notebook.Run{ID: "r1"})
	_ = nb.SaveOutput(notebook.Output{ID: "o1", Title: "Week 1"}, "# Week 1")
	_ = os.WriteFile(filepath.Join(nb.Dir, "audit.jsonl"), []byte(`{"kind":"run"}`+"\n"), 0o600)
	var buf bytes.Buffer
	if err := s.ExportNotebook(info.ID, outputs, &buf); err != nil {
		t.Fatal(err)
	}
	return info, buf.Bytes()
}

func TestExportLeavesOutHistoryAndPaths(t *testing.T) {
	s := newSvc(t, summary)
	_, zip := exportedNotebook(t, s, false)
	got, err := s.ImportNotebook(bytes.NewReader(zip), int64(len(zip)))
	if err != nil {
		t.Fatal(err)
	}
	nb, _ := s.OpenNotebook(got.ID)
	for _, gone := range []string{notebook.RunsDir, "audit.jsonl", notebook.OutputsDir, notebook.ThreadsDir} {
		if _, err := os.Stat(filepath.Join(nb.Dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should not be in the import", gone)
		}
	}
	var relink, copied int
	for _, c := range nb.Manifest.Context {
		switch {
		case c.NeedsRelink:
			relink++
			if c.Path != "" || c.Enabled {
				t.Errorf("relink item keeps a path or stays enabled: %+v", c)
			}
		case c.Path != "":
			copied++
			if b, err := os.ReadFile(nb.ContextPath(c)); err != nil || string(b) != "ship in March" {
				t.Errorf("copied file: %q %v", b, err)
			}
		}
	}
	if relink != 1 || copied != 1 {
		t.Fatalf("relink %d copied %d", relink, copied)
	}
	sch, _ := s.ListSchedules(got.ID)
	if len(sch) != 1 || sch[0].Enabled {
		t.Fatalf("schedules come in disabled: %+v", sch)
	}
	if docs, err := s.notebookDocs(context.Background(), nb); err != nil || len(docs) != 1 {
		t.Fatalf("a run reads only the copied file: %d %v", len(docs), err)
	}
}

func TestExportWithOutputsAndImportTwice(t *testing.T) {
	s := newSvc(t, summary)
	info, zip := exportedNotebook(t, s, true)
	a, err := s.ImportNotebook(bytes.NewReader(zip), int64(len(zip)))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.ImportNotebook(bytes.NewReader(zip), int64(len(zip)))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == info.ID || b.ID == a.ID || !strings.HasSuffix(a.Name, "(imported)") {
		t.Fatalf("ids %s %s %s, name %q", info.ID, a.ID, b.ID, a.Name)
	}
	nb, _ := s.OpenNotebook(b.ID)
	if nb.Manifest.ID != b.ID {
		t.Fatalf("manifest id %q, folder %q", nb.Manifest.ID, b.ID)
	}
	if outs, _ := nb.ListOutputs(); len(outs) != 1 {
		t.Fatalf("outputs: %d", len(outs))
	}
}

func TestRelinkContext(t *testing.T) {
	s := newSvc(t, summary)
	_, zip := exportedNotebook(t, s, false)
	got, _ := s.ImportNotebook(bytes.NewReader(zip), int64(len(zip)))
	nb, _ := s.OpenNotebook(got.ID)
	var id string
	for _, c := range nb.Manifest.Context {
		if c.NeedsRelink {
			id = c.ID
		}
	}
	if _, err := s.RelinkContext(got.ID, id, "relative/path"); !IsInvalid(err) {
		t.Fatalf("relative path: %v", err)
	}
	folder := t.TempDir()
	info, err := s.RelinkContext(got.ID, id, folder)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range info.Context {
		if c.ID == id && (c.NeedsRelink || c.Path != folder || !c.Enabled) {
			t.Fatalf("after relink: %+v", c)
		}
	}
}

func TestResearchArchiveKeepsNotesDropsChats(t *testing.T) {
	s := newSvc(t, summary)
	r, _ := s.CreateResearch("Vendors", "Pick one")
	if _, err := s.SaveResearchNote(r.ID, research.Note{Title: "Audit", Text: "Check the contract"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateResearchChat(r.ID, "Q1"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.ExportResearch(r.ID, &buf); err != nil {
		t.Fatal(err)
	}
	got, err := s.ImportResearch(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if notes, _ := s.ResearchNotes(got.ID); len(notes) != 1 {
		t.Fatalf("notes: %d", len(notes))
	}
	if chats, _ := s.ListResearchChats(got.ID); len(chats) != 0 {
		t.Fatalf("chats must not travel: %d", len(chats))
	}
	if got.ID == r.ID {
		t.Fatal("import must not overwrite the original")
	}
}

func TestImportRefusesAWrongKind(t *testing.T) {
	s := newSvc(t, summary)
	r, _ := s.CreateResearch("Vendors", "")
	var buf bytes.Buffer
	_ = s.ExportResearch(r.ID, &buf)
	if _, err := s.ImportNotebook(bytes.NewReader(buf.Bytes()), int64(buf.Len())); !IsInvalid(err) {
		t.Fatalf("a research archive imported as a notebook: %v", err)
	}
	if _, err := s.ImportNotebook(strings.NewReader("junk"), 4); !IsInvalid(err) {
		t.Fatalf("junk: %v", err)
	}
}

func TestArchiveIsAudited(t *testing.T) {
	s := newSvc(t, summary)
	s.UserAuditPath = filepath.Join(t.TempDir(), "audit.jsonl")
	_, zip := exportedNotebook(t, s, false)
	if _, err := s.ImportNotebook(bytes.NewReader(zip), int64(len(zip))); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.UserAuditPath)
	log := string(b)
	if !strings.Contains(log, auditExport) || !strings.Contains(log, auditImport) {
		t.Fatalf("audit: %s", log)
	}
	if strings.Contains(log, string(filepath.Separator)+"notes.md") {
		t.Fatalf("audit must not hold paths: %s", log)
	}
}
