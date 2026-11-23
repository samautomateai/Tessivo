package service

import (
	"context"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

func TestDiffAVersionWithItsParent(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	first, err := s.Run(context.Background(), RunRequest{Notebook: nb, Prompt: "x", Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	o, md, _ := nb.GetOutput(first.OutputID)
	child := notebook.Output{
		ID: notebook.NewID(), RunID: o.RunID, WorkType: o.WorkType, Title: o.Title + " v2",
		CreatedAt: o.CreatedAt, Data: o.Data, ParentID: o.ID,
	}
	if err := nb.SaveOutput(child, md+"\nA new closing line.\n"); err != nil {
		t.Fatal(err)
	}
	d, err := s.DiffOutputs(info.ID, child.ID, "")
	if err != nil || d.From != o.ID || d.To != child.ID || d.Counts.Added == 0 {
		t.Fatalf("%v %+v", err, d.Counts)
	}
	if _, err := s.DiffOutputs(info.ID, o.ID, ""); !IsInvalid(err) {
		t.Fatalf("an original has no parent: %v", err)
	}
	if d, err := s.DiffOutputs(info.ID, o.ID, child.ID); err != nil || d.Counts.Removed == 0 {
		t.Fatalf("explicit against: %v %+v", err, d.Counts)
	}
}
