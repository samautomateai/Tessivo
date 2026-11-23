package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRun stands in for the packaging tools; check, when set, inspects what
// the tool would see while it runs.
var (
	ranTools []string
	check    func(name string, args []string) error
)

func fakeRun(_ context.Context, _, name string, args ...string) error {
	ranTools = append(ranTools, name+" "+strings.Join(args, " "))
	if check != nil {
		return check(name, args)
	}
	return nil
}

// bundleFixture makes a fake Flutter bundle for platform with the given files.
func bundleFixture(t *testing.T, root, platform string, files ...string) string {
	t.Helper()
	b := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(desktopTargets[platform].bundle, "{arch}", "x64")))
	for _, f := range files {
		p := filepath.Join(b, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(f), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	icon := filepath.Join(root, filepath.FromSlash(iconPath))
	if err := os.MkdirAll(filepath.Dir(icon), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(icon, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func reset(t *testing.T) {
	t.Helper()
	ranTools, check = nil, nil
	t.Cleanup(func() { ranTools, check = nil, nil })
}

// arMembers parses an ar archive into member name → body.
func arMembers(t *testing.T, b []byte) ([]string, map[string][]byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("!<arch>\n")) {
		t.Fatal("not an ar archive")
	}
	b = b[8:]
	var order []string
	out := map[string][]byte{}
	for len(b) >= 60 {
		h := string(b[:60])
		name := strings.TrimSuffix(strings.TrimSpace(h[:16]), "/")
		var size int
		for _, c := range strings.TrimSpace(h[48:58]) {
			size = size*10 + int(c-'0')
		}
		if h[58:60] != "`\n" {
			t.Fatalf("bad ar header %q", h)
		}
		order = append(order, name)
		out[name] = b[60 : 60+size]
		b = b[60+size+size%2:]
	}
	return order, out
}

func tarEntries(t *testing.T, gz []byte) map[string]*tar.Header {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(zr)
	out := map[string]*tar.Header{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		out[h.Name] = h
	}
}

func TestLinuxInstallers(t *testing.T) {
	reset(t)
	a, _, root := newApp(t)
	bundleFixture(t, root, "linux", "tessivo_desktop", "tess", "lib/libapp.so", "data/icudtl.dat")
	check = func(name string, args []string) error {
		if _, err := os.Stat(filepath.Join(args[1], "AppRun")); err != nil {
			t.Errorf("AppDir lacks AppRun: %v", err)
		}
		if _, err := os.Stat(filepath.Join(args[1], "lib", "libapp.so")); err != nil {
			t.Errorf("AppDir lacks the bundle: %v", err)
		}
		return nil
	}
	dist := filepath.Join(root, "dist")
	if err := a.installer(context.Background(), "v1.2.3", dist, "linux", "amd64", ""); err != nil {
		t.Fatal(err)
	}
	if len(ranTools) != 1 || !strings.HasPrefix(ranTools[0], "appimagetool --no-appstream ") ||
		!strings.HasSuffix(ranTools[0], "tessivo-desktop_v1.2.3_linux_amd64.AppImage") {
		t.Fatalf("appimagetool: %q", ranTools)
	}
	deb, err := os.ReadFile(filepath.Join(dist, "tessivo-desktop_v1.2.3_linux_amd64.deb"))
	if err != nil {
		t.Fatal(err)
	}
	order, m := arMembers(t, deb)
	if strings.Join(order, ",") != "debian-binary,control.tar.gz,data.tar.gz" || string(m["debian-binary"]) != "2.0\n" {
		t.Fatalf("deb members: %v", order)
	}
	ctl := tarEntries(t, m["control.tar.gz"])
	if ctl["./control"] == nil {
		t.Fatal("no control file")
	}
	data := tarEntries(t, m["data.tar.gz"])
	if h := data["./opt/tessivo/tessivo_desktop"]; h == nil || h.Mode != 0o755 || h.Uid != 0 {
		t.Fatalf("app binary: %+v", h)
	}
	if h := data["./opt/tessivo/lib/libapp.so"]; h == nil || h.Mode != 0o644 {
		t.Fatalf("library: %+v", h)
	}
	if h := data["./usr/bin/tess"]; h == nil || h.Linkname != "/opt/tessivo/tess" {
		t.Fatalf("tess link: %+v", h)
	}
	if data["./usr/share/applications/"+appID+".desktop"] == nil ||
		data["./usr/share/icons/hicolor/256x256/apps/tessivo.png"] == nil {
		t.Fatal("menu entry or icon missing")
	}
}

func TestDebControl(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tess"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out.deb")
	if err := writeDeb(dest, dir, "dev-abc123", "arm64", []byte("png")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dest)
	_, m := arMembers(t, b)
	zr, _ := gzip.NewReader(bytes.NewReader(m["control.tar.gz"]))
	tr := tar.NewReader(zr)
	_, _ = tr.Next()
	body, _ := io.ReadAll(tr)
	for _, want := range []string{"Version: 0.0.0~dev.abc123\n", "Architecture: arm64\n", "Package: tessivo\n"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("control lacks %q:\n%s", want, body)
		}
	}
	if err := writeDeb(dest, dir, "v1", "mips", nil); err == nil {
		t.Fatal("unknown arch must fail")
	}
	for in, want := range map[string]string{"v1.2.3": "1.2.3", "1.0.0-rc1": "1.0.0-rc1", "dev-9f": "0.0.0~dev.9f"} {
		if got := debVersion(in); got != want {
			t.Fatalf("debVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowsInstaller(t *testing.T) {
	reset(t)
	a, _, root := newApp(t)
	bundleFixture(t, root, "windows", "tessivo_desktop.exe", "tess.exe", "LICENSE")
	var script string
	check = func(name string, args []string) error {
		b, err := os.ReadFile(args[1])
		script = string(b)
		return err
	}
	dist := filepath.Join(root, "dist")
	if err := a.installer(context.Background(), "v2.0.0", dist, "windows", "amd64", `C:\Inno\iscc.exe`); err != nil {
		t.Fatal(err)
	}
	if len(ranTools) != 1 || !strings.HasPrefix(ranTools[0], `C:\Inno\iscc.exe /Q `) {
		t.Fatalf("iscc: %q", ranTools)
	}
	for _, want := range []string{
		"AppVersion=v2.0.0\r\n", "OutputBaseFilename=tessivo-desktop_v2.0.0_windows_amd64_setup\r\n",
		"ArchitecturesAllowed=x64compatible\r\n", "PrivilegesRequired=lowest\r\n", "AppId=" + innoAppID,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script lacks %q:\n%s", want, script)
		}
	}
	if _, err := os.Stat(filepath.Join(dist, "tessivo-desktop_v2.0.0_windows_amd64.iss")); !os.IsNotExist(err) {
		t.Fatalf("the script must be removed after the build: %v", err)
	}
}

func TestMacInstaller(t *testing.T) {
	reset(t)
	a, _, root := newApp(t)
	bundle := bundleFixture(t, root, "macos", "Contents/MacOS/tessivo_desktop")
	check = func(name string, args []string) error {
		stage := args[4]
		if _, err := os.Stat(filepath.Join(stage, "Tessivo.app", "Contents", "MacOS", "tessivo_desktop")); err != nil {
			t.Errorf("the image must hold Tessivo.app: %v", err)
		}
		return errors.New("hdiutil failed")
	}
	err := a.installer(context.Background(), "v1", filepath.Join(root, "dist"), "macos", "arm64", "")
	if err == nil || !strings.HasPrefix(ranTools[0], "hdiutil create -volname Tessivo -srcfolder ") {
		t.Fatalf("hdiutil: %v %q", err, ranTools)
	}
	if _, err := os.Stat(filepath.Join(bundle, "Contents", "MacOS", "tessivo_desktop")); err != nil {
		t.Fatalf("the bundle must be moved back after the build: %v", err)
	}
	if err := a.installer(context.Background(), "v1", root, "windows", "amd64", ""); err == nil {
		t.Fatal("a missing bundle must fail")
	}
}

func TestRunInstallerUsage(t *testing.T) {
	var errOut bytes.Buffer
	if code := run(context.Background(), []string{"installer", "-version", "v1"}, &errOut, &fakeGo{}); code != 2 {
		t.Fatalf("missing -platform: code %d", code)
	}
	if !strings.Contains(errOut.String(), "installer") {
		t.Fatalf("usage must mention installer: %s", errOut.String())
	}
}
