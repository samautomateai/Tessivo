package notebook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameAndArchive(t *testing.T) {
	nb, err := Create(filepath.Join(t.TempDir(), "nb"), "Old", "analyze")
	if err != nil {
		t.Fatal(err)
	}
	if err := nb.Rename("  New name  "); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", MaxName+1), "a\nb"} {
		if err := nb.Rename(bad); err == nil {
			t.Errorf("rename to %q accepted", bad)
		}
	}
	if err := nb.SetArchived(true); err != nil {
		t.Fatal(err)
	}
	got, err := Open(nb.Dir)
	if err != nil || got.Manifest.Name != "New name" || !got.Manifest.Archived {
		t.Fatalf("%v %+v", err, got.Manifest)
	}
}

func TestRemoveDeletesOnlyNotebooks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nb")
	if _, err := Create(dir, "", "summarize"); err != nil {
		t.Fatal(err)
	}
	plain := t.TempDir()
	_ = os.WriteFile(filepath.Join(plain, "keep.txt"), []byte("mine"), 0o600)
	if err := Remove(plain); err == nil {
		t.Fatal("removed a folder that is not a notebook")
	}
	if _, err := os.Stat(filepath.Join(plain, "keep.txt")); err != nil {
		t.Fatal("a refused remove touched files")
	}
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("still there: %v", err)
	}
}

func TestRemoveRefusesBusyNotebooks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nb")
	nb, err := Create(dir, "", "analyze")
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := nb.LockRun("20260101T000000-aaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("running: %v", err)
	}
	unlock()
	if err := nb.SaveCheckpoint("20260101T000000-bbbbbb", map[string]string{"status": "waiting"}); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("paused run: %v", err)
	}
	if err := nb.RemoveCheckpoint("20260101T000000-bbbbbb"); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
}
