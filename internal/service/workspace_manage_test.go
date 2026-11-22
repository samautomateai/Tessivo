package service

import (
	"errors"
	"testing"
)

func TestUpdateAndDeleteNotebook(t *testing.T) {
	s := newSvc(t, summary)
	info, err := s.CreateNotebook("Ops", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	name, archived := "Weekly ops", true
	got, err := s.UpdateNotebook(info.ID, NotebookUpdate{Name: &name, Archived: &archived})
	if err != nil || got.Name != name || !got.Archived {
		t.Fatalf("%v %+v", err, got)
	}
	if list, _ := s.ListNotebooks(); len(list) != 1 || !list[0].Archived {
		t.Fatalf("list: %+v", list)
	}
	empty := " "
	if _, err := s.UpdateNotebook(info.ID, NotebookUpdate{Name: &empty}); !IsInvalid(err) {
		t.Fatalf("blank name: %v", err)
	}
	if err := s.DeleteNotebook(info.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Describe(info.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
	if err := s.DeleteNotebook(info.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestDeleteBusyNotebookIsAConflict(t *testing.T) {
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	nb, _ := s.OpenNotebook(info.ID)
	unlock, err := nb.LockRun("20260101T000000-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if err := s.DeleteNotebook(info.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("want conflict, got %v", err)
	}
}
