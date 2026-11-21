package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// fakeGo records go builds and writes a fake binary where -o points.
type fakeGo struct{ builds []build }

func (f *fakeGo) build(_ context.Context, b build) error {
	f.builds = append(f.builds, b)
	if err := os.MkdirAll(filepath.Dir(b.out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(b.out, []byte("bin-"+b.goos+"-"+b.goarch), 0o755)
}

func newApp(t *testing.T) (*app, *fakeGo, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Tessivo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGo{}
	return &app{root: root, goBuild: g.build, out: io.Discard}, g, root
}

func names(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	if strings.HasSuffix(path, ".zip") {
		out = zipNames(t, path)
	} else {
		out = tarNames(t, path)
	}
	sort.Strings(out)
	return out
}

func zipNames(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	for _, f := range r.File {
		out = append(out, f.Name)
	}
	return out
}

func tarNames(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		name := h.Name
		if h.Typeflag == tar.TypeSymlink {
			name += " -> " + h.Linkname
		}
		out = append(out, name)
	}
	return out
}

func TestCLIBuildsEveryTarget(t *testing.T) {
	a, g, root := newApp(t)
	dist := filepath.Join(root, "dist")
	if err := a.cli(context.Background(), "v1.2.3", dist); err != nil {
		t.Fatal(err)
	}
	if len(g.builds) != len(cliTargets) {
		t.Fatalf("%d builds, want %d", len(g.builds), len(cliTargets))
	}
	for _, b := range g.builds {
		if b.pkg != "./cmd/tess" || !strings.Contains(b.ldflags, "internal/cli.Version=v1.2.3") {
			t.Fatalf("build: %+v", b)
		}
	}
	got := names(t, filepath.Join(dist, "tess_v1.2.3_windows_amd64.zip"))
	want := []string{"tess_v1.2.3_windows_amd64/README.md", "tess_v1.2.3_windows_amd64/tess.exe"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("windows archive: %v", got)
	}
	got = names(t, filepath.Join(dist, "tess_v1.2.3_darwin_arm64.tar.gz"))
	if len(got) != 2 || got[1] != "tess_v1.2.3_darwin_arm64/tess" {
		t.Fatalf("macOS archive: %v", got)
	}
}

func TestDesktopBundlesTess(t *testing.T) {
	a, g, root := newApp(t)
	linux := strings.ReplaceAll(desktopTargets["linux"].bundle, "{arch}", "x64")
	bundle := filepath.Join(root, filepath.FromSlash(linux), "lib")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "libapp.so"), []byte("so"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("libapp.so", filepath.Join(bundle, "libapp.so.1")); err != nil {
		t.Logf("symlinks unavailable here (%v); symlink entry not checked", err)
	}
	dist := filepath.Join(root, "dist")
	if err := a.desktop(context.Background(), "v1.0.0", dist, "linux", "amd64"); err != nil {
		t.Fatal(err)
	}
	if b := g.builds[0]; b.goos != "linux" || b.goarch != "amd64" {
		t.Fatalf("tess must be built for the bundle's platform: %+v", b)
	}
	got := strings.Join(names(t, filepath.Join(dist, "tessivo-desktop_v1.0.0_linux_amd64.tar.gz")), "\n")
	for _, want := range []string{"tessivo-desktop_v1.0.0_linux_amd64/tess", "lib/libapp.so"} {
		if !strings.Contains(got, want) {
			t.Fatalf("archive lacks %s:\n%s", want, got)
		}
	}
	if fi, err := os.Lstat(filepath.Join(bundle, "libapp.so.1")); err == nil && fi.Mode()&os.ModeSymlink != 0 &&
		!strings.Contains(got, "libapp.so.1 -> libapp.so") {
		t.Fatalf("symlinks must stay symlinks:\n%s", got)
	}
}

func TestDesktopMacPutsTessInsideApp(t *testing.T) {
	a, g, root := newApp(t)
	app := filepath.Join(root, filepath.FromSlash(desktopTargets["macos"].bundle))
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.desktop(context.Background(), "v1", filepath.Join(root, "dist"), "macos", "arm64"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(g.builds[0].out), "tessivo_desktop.app/Contents/MacOS/tess") {
		t.Fatalf("tess must sit next to the app's executable: %s", g.builds[0].out)
	}
}

func TestDesktopErrors(t *testing.T) {
	a, _, root := newApp(t)
	if err := a.desktop(context.Background(), "v1", root, "beos", "amd64"); err == nil {
		t.Fatal("unknown platform must fail")
	}
	if err := a.desktop(context.Background(), "v1", root, "windows", "amd64"); err == nil ||
		!strings.Contains(err.Error(), "flutter build windows") {
		t.Fatalf("a missing bundle must say how to build it: %v", err)
	}
}

func TestSums(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"b.zip": "bbb", "a.tar.gz": "aaa"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := sums(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "  a.tar.gz") ||
		!strings.HasPrefix(lines[0], "9834876dcfb05cb167a5c24953eba58c4ac89b1adf57f28f2f9d09af107ee8f0") {
		t.Fatalf("sums:\n%s", b)
	}
	if err := sums(dir); err != nil {
		t.Fatal(err)
	}
	if b2, _ := os.ReadFile(filepath.Join(dir, "SHA256SUMS")); !bytes.Equal(b, b2) {
		t.Fatal("running sums twice must not list SHA256SUMS itself")
	}
}

func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	cases := [][]string{{}, {"nope"}, {"cli"}, {"desktop", "-version", "v1"}, {"sums"}}
	for _, args := range cases {
		if code := run(context.Background(), args, &out, &fakeGo{}); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}

func TestRunSumsAndCLI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.zip"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run(context.Background(), []string{"sums", "-dir", dir}, &out, &fakeGo{}); code != 0 {
		t.Fatalf("sums: exit %d: %s", code, &out)
	}
	t.Chdir(t.TempDir()) // no README.md here: the CLI build must fail cleanly
	if code := run(context.Background(), []string{"cli", "-version", "v1", "-out", dir}, &out, &fakeGo{}); code != 1 {
		t.Fatalf("cli without README: exit %d", code)
	}
}

func TestOSGoBuildReportsFailure(t *testing.T) {
	b := newBuild(t.TempDir(), "v1", target{"linux", "amd64"}, filepath.Join(t.TempDir(), "tess"))
	b.pkg = "./does-not-exist"
	err := osGo{}.build(context.Background(), b)
	if err == nil || !strings.Contains(err.Error(), "go build linux/amd64") {
		t.Fatalf("want a go build error, got %v", err)
	}
}

func TestZipRefusesSymlinks(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a", filepath.Join(src, "b")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	dest := filepath.Join(t.TempDir(), "x.zip")
	if err := archive(src, dest, "windows"); err == nil || !strings.Contains(err.Error(), "symlinks") {
		t.Fatalf("want a symlink error, got %v", err)
	}
	if _, err := os.Stat(dest + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("a failed archive must not leave a temp file")
	}
}
