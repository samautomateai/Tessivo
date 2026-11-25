package watch

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFingerprintChangesOnAddRemoveAndEdit(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.md", "one")
	fp := func() string {
		t.Helper()
		s, err := Fingerprint(dir)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	base := fp()
	if fp() != base {
		t.Fatal("a fingerprint must be stable")
	}
	write("b.md", "two")
	added := fp()
	if added == base {
		t.Fatal("adding a file must change it")
	}
	write("a.md", "one, longer")
	if fp() == added {
		t.Fatal("changing a file's size must change it")
	}
	edited := fp()
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.md"), later, later); err != nil {
		t.Fatal(err)
	}
	if fp() == edited {
		t.Fatal("changing a file's time must change it")
	}
	if err := os.Remove(filepath.Join(dir, "b.md")); err != nil {
		t.Fatal(err)
	}
	if fp() == edited {
		t.Fatal("removing a file must change it")
	}
}

func TestFingerprintOfAFileAndOfMissing(t *testing.T) {
	f := filepath.Join(t.TempDir(), "x.txt")
	_ = os.WriteFile(f, []byte("x"), 0o600)
	if s, err := Fingerprint(f); err != nil || s == "" {
		t.Fatalf("%q %v", s, err)
	}
	if _, err := Fingerprint(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Fatal("a missing path is an error")
	}
}

func TestFingerprintIsBounded(t *testing.T) {
	dir := t.TempDir()
	defer func(n int) { maxFiles = n }(maxFiles)
	maxFiles = 3
	for i := range 5 {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.txt", i)), []byte("x"), 0o600)
	}
	if _, err := Fingerprint(dir); err != nil {
		t.Fatalf("a big folder is fingerprinted from its first files, not refused: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	before, _ := Fingerprint(dir)
	_ = os.WriteFile(filepath.Join(dir, ".git", "index"), []byte("x"), 0o600)
	if after, _ := Fingerprint(dir); after != before {
		t.Fatal(".git is not watched")
	}
}

func TestFingerprintBoundsFoldersToo(t *testing.T) {
	dir := t.TempDir()
	defer func(n int) { maxEntries = n }(maxEntries)
	maxEntries = 3
	for i := range 5 {
		if err := os.MkdirAll(filepath.Join(dir, fmt.Sprintf("d%d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	before, err := Fingerprint(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "d4", "late.txt"), []byte("x"), 0o600)
	if after, _ := Fingerprint(dir); after != before {
		t.Fatal("the walk stops after maxEntries entries, folders included")
	}
}
