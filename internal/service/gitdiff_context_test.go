package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitChangesAreReadOnEveryRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@example.com"},
			args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	_ = os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0o600)
	run("add", ".")
	run("commit", "-q", "-m", "init")
	_ = os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n\nfunc Retry() {}\n"), 0o600)

	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Review", "analyze")
	got, err := s.AddGitDiffContext(info.ID, repo, "main", "")
	if err != nil || got.Context[0].Kind != "git-diff" ||
		!strings.Contains(got.Context[0].Label, "changes since main") {
		t.Fatalf("%v %+v", err, got.Context)
	}
	nb, _ := s.OpenNotebook(info.ID)
	docs, err := s.notebookDocs(context.Background(), nb)
	if err != nil || len(docs) != 1 || !strings.Contains(docs[0].Text, "func Retry") {
		t.Fatalf("%v %+v", err, docs)
	}
	if _, err := s.AddGitDiffContext(info.ID, repo, "--output=x", ""); !IsInvalid(err) {
		t.Fatalf("bad base: %v", err)
	}
}
