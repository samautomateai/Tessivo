package service

import (
	"testing"
)

func TestTagsAndPins(t *testing.T) {
	s := newSvc(t, summary)
	a, _ := s.CreateNotebook("Alpha", "summarize")
	b, _ := s.CreateNotebook("Beta", "summarize")
	tags, pinned := []string{"Infra", "infra", "Q3 review"}, true
	got, err := s.UpdateNotebook(b.ID, NotebookUpdate{Tags: &tags, Pinned: &pinned})
	if err != nil || !got.Pinned || len(got.Tags) != 2 || got.Tags[0] != "Infra" {
		t.Fatalf("%v %+v", err, got)
	}
	list, _ := s.ListNotebooks()
	if list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("pinned first: %v, %v", list[0].Name, list[1].Name)
	}
	tagged, err := s.ListNotebooksTagged("INFRA")
	if err != nil || len(tagged) != 1 || tagged[0].ID != b.ID {
		t.Fatalf("tagged: %v %+v", err, tagged)
	}
	bad := []string{"a/b"}
	if _, err := s.UpdateNotebook(a.ID, NotebookUpdate{Tags: &bad}); !IsInvalid(err) {
		t.Fatalf("bad tag: %v", err)
	}
	hits, _ := s.Search("q3", 10)
	if len(hits) == 0 || hits[0].NotebookID != b.ID {
		t.Fatalf("search by tag: %+v", hits)
	}
}

func TestResearchTagsAndPins(t *testing.T) {
	s := newSvc(t, summary)
	a, _ := s.CreateResearch("Alpha", "")
	b, _ := s.CreateResearch("Beta", "")
	tags, pinned := []string{"vendors"}, true
	got, err := s.OrganizeResearch(b.ID, &tags, &pinned)
	if err != nil || !got.Pinned || len(got.Tags) != 1 {
		t.Fatalf("%v %+v", err, got)
	}
	list, _ := s.ListResearch()
	if list[0].ID != b.ID || list[1].ID != a.ID {
		t.Fatalf("pinned first: %+v", list)
	}
}
