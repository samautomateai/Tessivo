package pack

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/gitdiff"
)

func TestKindOf(t *testing.T) {
	folder := teamPack(t)
	zipFile := filepath.Join(t.TempDir(), "team.zip")
	_ = os.WriteFile(zipFile, []byte("PK"), 0o600)
	good := map[string]string{
		"https://github.com/acme/tessivo-pack.git": KindGit,
		"git@github.com:acme/tessivo-pack.git":     KindGit,
		folder:                                     KindFolder,
		zipFile:                                    KindZip,
	}
	for src, want := range good {
		if got, err := KindOf(src); err != nil || got != want {
			t.Errorf("KindOf(%s) = %q, %v; want %q", src, got, err, want)
		}
	}
	for _, src := range []string{
		"file:///tmp/pack", "ext::sh -c touch% /tmp/pwned", "http://example.com/p.git",
		filepath.Join(folder, "missing"), "relative/pack",
	} {
		if _, err := KindOf(src); err == nil {
			t.Errorf("KindOf(%q): want an error", src)
		}
	}
}

func zipPack(t *testing.T, root string, extra map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	src := teamPack(t)
	_ = filepath.WalkDir(src, func(p string, d os.DirEntry, _ error) error {
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(src, p)
			w, _ := zw.Create(root + filepath.ToSlash(rel))
			b, _ := os.ReadFile(p)
			_, _ = w.Write(b)
		}
		return nil
	})
	for name, body := range extra {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	file := filepath.Join(t.TempDir(), "pack.zip")
	_ = os.WriteFile(file, buf.Bytes(), 0o600)
	return file
}

func TestFetchZip(t *testing.T) {
	for _, root := range []string{"", "team-pack/"} {
		dest := filepath.Join(t.TempDir(), "team")
		if err := Fetch(context.Background(), KindZip, zipPack(t, root, nil), "", dest); err != nil {
			t.Fatalf("root %q: %v", root, err)
		}
		if p, err := Load(dest); err != nil || p.Manifest.ID != "team" {
			t.Fatalf("root %q: %v %+v", root, err, p.Manifest)
		}
	}
	dest := filepath.Join(t.TempDir(), "evil")
	err := Fetch(context.Background(), KindZip, zipPack(t, "", map[string]string{"../x": "x"}), "", dest)
	if err == nil {
		t.Fatal("a zip that escapes must fail")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatal("nothing may be left after a refused zip")
	}
}

// gitRepo commits the team pack into a fresh repository and returns it.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := teamPack(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "pack"},
	} {
		if _, err := gitdiff.Run(ctx, repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func TestFetchAndRefreshGit(t *testing.T) {
	repo := gitRepo(t)
	defer func(v bool) { allowLocalGit = v }(allowLocalGit)
	allowLocalGit = true
	ctx := context.Background()
	dest := filepath.Join(t.TempDir(), "team")
	if err := Fetch(ctx, KindGit, repo, "main", dest); err != nil {
		t.Fatal(err)
	}
	if p, err := Load(dest); err != nil || p.Manifest.Name != "Team pack" {
		t.Fatalf("%v %+v", err, p.Manifest)
	}
	write(t, repo, ManifestFile, Manifest{FormatVersion: 1, ID: "team", Name: "Team pack v2"})
	if _, err := gitdiff.Run(ctx, repo, "-c", "user.name=t", "-c", "user.email=t@example.com",
		"commit", "-q", "-am", "v2"); err != nil {
		t.Fatal(err)
	}
	if err := Refresh(ctx, dest, "main"); err != nil {
		t.Fatal(err)
	}
	if p, _ := Load(dest); p.Manifest.Name != "Team pack v2" {
		t.Fatalf("after refresh: %q", p.Manifest.Name)
	}
	if err := Refresh(ctx, dest, "bad..branch"); err == nil || !strings.Contains(err.Error(), "branch") {
		t.Fatalf("bad branch: %v", err)
	}
}
