package service

import (
	"context"
	"errors"
	"testing"
)

func TestRerunAnOutputOnAnotherModel(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	first, err := s.Run(context.Background(), RunRequest{
		Notebook: nb, Prompt: "Summarize {{week}}", Vars: map[string]string{"week": "47"}, Save: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ri, err := s.OutputRun(info.ID, first.OutputID)
	if err != nil || ri.Prompt != "Summarize {{week}}" || ri.Vars["week"] != "47" || ri.Model != "litellm/m" {
		t.Fatalf("%v %+v", err, ri)
	}
	req, err := s.RerunRequest(info.ID, first.OutputID, "litellm/other")
	if err != nil || req.Overrides.Model != "litellm/other" || req.Prompt != ri.Prompt || !req.Save {
		t.Fatalf("%v %+v", err, req)
	}
	again, err := s.Run(context.Background(), req, nil)
	if err != nil || again.OutputID == "" || again.OutputID == first.OutputID {
		t.Fatalf("%v %+v", err, again)
	}
	if _, err := s.OutputRun(info.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing output: %v", err)
	}
}
