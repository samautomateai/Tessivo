package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"},
		args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	_ = os.WriteFile(filepath.Join(dir, "pay.go"), []byte("package pay\n\nfunc Charge() {}\n"), 0o600)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "init")
	git(t, dir, "checkout", "-q", "-b", "feature")
	src := "package pay\n\nfunc Charge(idempotencyKey string) {}\n"
	_ = os.WriteFile(filepath.Join(dir, "pay.go"), []byte(src), 0o600)
	git(t, dir, "commit", "-q", "-am", "add key")
	_ = os.WriteFile(filepath.Join(dir, "notes.md"), []byte("uncommitted\n"), 0o600)
	git(t, dir, "add", "notes.md")
	return dir
}

func TestDiffAgainstABase(t *testing.T) {
	dir := repo(t)
	d, err := Diff(context.Background(), dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"idempotencyKey", "pay.go", "uncommitted", "=== changes since main", "=== not yet committed",
	} {
		if !strings.Contains(d, want) {
			t.Errorf("diff lacks %q:\n%s", want, d)
		}
	}
}

func TestRefusals(t *testing.T) {
	dir := repo(t)
	for _, base := range []string{"", "-p", "--output=/tmp/x", "a..b", "main;rm", strings.Repeat("a", 300)} {
		if _, err := Diff(context.Background(), dir, base); err == nil {
			t.Errorf("base %q accepted", base)
		}
	}
	if _, err := Diff(context.Background(), t.TempDir(), "main"); err == nil {
		t.Error("a folder that is not a repository was read")
	}
	if _, err := Diff(context.Background(), "relative/path", "main"); err == nil {
		t.Error("a relative path was accepted")
	}
}

func TestRepoConfigCannotRunPrograms(t *testing.T) {
	dir := repo(t)
	marker := filepath.Join(t.TempDir(), "ran")
	hook := "touch " + filepath.ToSlash(marker)
	git(t, dir, "config", "core.fsmonitor", hook)
	git(t, dir, "config", "diff.external", hook)
	if _, err := Diff(context.Background(), dir, "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the repository's config ran a program")
	}
}
