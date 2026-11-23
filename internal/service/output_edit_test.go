package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestEditOutputSavesAVersion(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Board", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	res, err := s.Run(context.Background(), RunRequest{Notebook: nb, Prompt: "Summarize", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(nb.Dir, notebook.OutputsDir, res.OutputID+".json")
	before, _ := os.ReadFile(parentFile)

	edited := `{"title":"Board digest, fixed","tldr":"t","key_points":["k","another"],` +
		`"per_source":[],"action_items":[]}`
	got, err := s.EditOutput(info.ID, res.OutputID, json.RawMessage(edited))
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentID != res.OutputID || got.Version != 2 || got.Title != "Board digest, fixed" {
		t.Fatalf("version: %+v", got.Output)
	}
	if !strings.Contains(got.Markdown, "another") {
		t.Fatalf("the edit must be rendered:\n%s", got.Markdown)
	}
	if after, _ := os.ReadFile(parentFile); string(after) != string(before) {
		t.Fatal("the parent output must not change")
	}
	log, _ := os.ReadFile(s.AuditPath(nb))
	if !strings.Contains(string(log), `"edited_from":"`+res.OutputID+`"`) || strings.Contains(string(log), "another") {
		t.Fatalf("the edit is audited by digest, without its content:\n%s", log)
	}

	for name, bad := range map[string]string{
		"not JSON":       `{"title":`,
		"wrong shape":    `{"title":"x"}`,
		"nothing new":    edited,
		"wrong type":     `{"title":1,"tldr":"t","key_points":[],"per_source":[],"action_items":[]}`,
		"over the limit": `"` + strings.Repeat("x", MaxEditedOutput) + `"`,
	} {
		if _, err := s.EditOutput(info.ID, got.ID, json.RawMessage(bad)); !IsInvalid(err) {
			t.Errorf("%s: want invalid, got %v", name, err)
		}
	}
	if _, err := s.EditOutput(info.ID, "missing", json.RawMessage(edited)); err == nil {
		t.Fatal("a missing output must fail")
	}
}
