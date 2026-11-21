package agenttools

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkfile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestResolveRefusesEscapes(t *testing.T) {
	root := t.TempDir()
	other := t.TempDir()
	mkfile(t, filepath.Join(root, "a.txt"), "a")
	secret := mkfile(t, filepath.Join(other, "secret.txt"), "s")
	r, err := NewRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	escapes := map[string]string{
		"parent":       "../x",
		"absolute":     secret,
		"drive letter": `C:\elsewhere\x`,
		"unc":          `\\host\share\x`,
		"slash unc":    "//host/share/x",
	}
	if err := os.Symlink(other, filepath.Join(root, "link")); err == nil {
		escapes["symlink"] = "link/secret.txt"
	} else {
		t.Logf("symlinks unavailable here (%v); symlink case skipped", err)
	}
	for name, p := range escapes {
		if _, err := r.Resolve(p); !errors.Is(err, ErrOutside) {
			t.Errorf("%s (%q): want ErrOutside, got %v", name, p, err)
		}
	}
	got, err := r.Resolve("sub/../a.txt")
	if err != nil || filepath.Base(got) != "a.txt" {
		t.Fatalf("inside path: %q, %v", got, err)
	}
	if _, err := r.Resolve("new/file.txt"); err != nil {
		t.Fatalf("a path that does not exist yet inside the root must resolve: %v", err)
	}
}

func TestRootsAllowSingleFiles(t *testing.T) {
	dir := t.TempDir()
	f := mkfile(t, filepath.Join(dir, "only.md"), "x")
	mkfile(t, filepath.Join(dir, "other.md"), "y")
	r, err := NewRoots(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve(f); err != nil {
		t.Fatalf("the root file itself: %v", err)
	}
	if _, err := r.Resolve(filepath.Join(dir, "other.md")); !errors.Is(err, ErrOutside) {
		t.Fatalf("a sibling of a root file: want ErrOutside, got %v", err)
	}
	if _, err := r.Resolve("only.md"); !errors.Is(err, ErrOutside) {
		t.Fatalf("relative paths need a folder root: want ErrOutside, got %v", err)
	}
	if len(r.Paths()) != 1 {
		t.Fatalf("paths: %v", r.Paths())
	}
}

func TestNewRootsErrors(t *testing.T) {
	if _, err := NewRoots(); err == nil {
		t.Fatal("no roots must be an error")
	}
	if _, err := NewRoots(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("a missing root must be an error")
	}
}

func TestResolveRefusesUNCBeforeTouchingIt(t *testing.T) {
	r, err := NewRoots(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	touched := false
	old := evalSymlinks
	evalSymlinks = func(p string) (string, error) {
		if strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
			touched = true
		}
		return old(p)
	}
	t.Cleanup(func() { evalSymlinks = old })
	for _, p := range []string{`\\attacker\share\x`, "//attacker/share/x"} {
		if _, err := r.Resolve(p); !errors.Is(err, ErrOutside) {
			t.Errorf("%q: want ErrOutside, got %v", p, err)
		}
	}
	if touched {
		t.Fatal("a UNC path must be refused before any filesystem access")
	}
}
