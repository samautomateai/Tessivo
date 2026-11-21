package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	goEnvLine    = "go env GOVERSION GOBIN GOPATH GOTOOLCHAIN"
	golangciGood = "golangci-lint has version 2.6.2 built with go1.27.4 from abc on 2026-11-01"
	goModText    = "module example.com/x\n\ngo 1.27.0\n\ntoolchain go1.27.4\n"
)

var fakeBin = filepath.FromSlash("/fakebin")

// fake answers commands from tables; go install makes the tool appear.
type fake struct {
	have      map[string]string // tool name -> output of any run of it
	installed map[string]string // tool name -> output once go install ran
	resp      map[string]string // command line -> stdout
	fail      map[string]error  // command line -> error
	calls     []string
}

func newFake() *fake {
	return &fake{
		have: map[string]string{
			"go": "", "git": "", "task": "Task version: v3.45.4", "golangci-lint": golangciGood,
			"govulncheck": "Scanner: govulncheck@v1.1.4", "gosec": "Version: 2.22.0",
			"flutter": "Flutter 3.38.0 • channel stable • https://github.com/flutter/flutter.git",
		},
		installed: map[string]string{},
		resp: map[string]string{
			goEnvLine:                         "go1.27.4\n" + fakeBin + "\n/gopath\nauto\n",
			"git config --get core.hooksPath": "scripts/hooks\n",
			"go env CGO_ENABLED":              "1\n",
		},
		fail: map[string]error{},
	}
}

func (f *fake) lookPath(name string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(name), ".exe")
	if _, ok := f.have[base]; ok {
		return filepath.Join(fakeBin, base), nil
	}
	return "", exec.ErrNotFound
}

func (f *fake) run(_ context.Context, c cmd) (string, error) {
	base := strings.TrimSuffix(filepath.Base(c.name), ".exe")
	line := strings.Join(append([]string{base}, c.args...), " ")
	f.calls = append(f.calls, line)
	if err, ok := f.fail[line]; ok {
		return "", err
	}
	if base == "go" && len(c.args) == 2 && c.args[0] == "install" {
		name := path.Base(strings.TrimSuffix(c.args[1], "@latest"))
		out, ok := f.installed[name]
		if !ok {
			out = f.have[name]
		}
		f.have[name] = out
		return "", nil
	}
	if line == "git config core.hooksPath scripts/hooks" {
		f.resp["git config --get core.hooksPath"] = "scripts/hooks\n"
	}
	if out, ok := f.resp[line]; ok {
		return out, nil
	}
	return f.have[base], nil
}

func (f *fake) ran(line string) bool {
	return slices.Contains(f.calls, line)
}

func newApp(t *testing.T, f *fake) (*app, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goModText), 0o600); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	a := &app{
		r: f, out: out, root: root, goos: "linux", timeout: cmdTimeout,
		getenv: func(k string) string {
			if k == "PATH" {
				return string(filepath.ListSeparator) + fakeBin
			}
			return ""
		},
	}
	return a, out
}

func TestDoctorAllGood(t *testing.T) {
	f := newFake()
	a, out := newApp(t, f)
	if code := run(context.Background(), a, []string{"doctor"}); code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, want := range []string{"all required tools are ready", "go1.27.4", "v3.45.4", "channel stable"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	for _, c := range f.calls {
		if strings.HasPrefix(c, "go install") {
			t.Errorf("healthy doctor must not install: %s", c)
		}
	}
}

func TestDoctorRepairsMissingToolAndHooks(t *testing.T) {
	f := newFake()
	delete(f.have, "gosec")
	f.installed["gosec"] = "Version: 2.22.1"
	f.resp["git config --get core.hooksPath"] = ""
	a, out := newApp(t, f)
	if code := run(context.Background(), a, []string{"doctor"}); code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	if !f.ran("go install github.com/securego/gosec/v2/cmd/gosec@latest") {
		t.Errorf("gosec not installed: %v", f.calls)
	}
	if !f.ran("git config core.hooksPath scripts/hooks") {
		t.Errorf("hooks not set: %v", f.calls)
	}
	if !strings.Contains(out.String(), "(repaired)") {
		t.Errorf("no repaired note:\n%s", out)
	}
}

func TestDoctorCheckOnlyReports(t *testing.T) {
	f := newFake()
	delete(f.have, "task")
	a, out := newApp(t, f)
	if code := run(context.Background(), a, []string{"doctor", "-check"}); code != 1 {
		t.Fatalf("code %d\n%s", code, out)
	}
	if f.ran("go install github.com/go-task/task/v3/cmd/task@latest") {
		t.Error("-check must not install")
	}
	if !strings.Contains(out.String(), "not installed") || !strings.Contains(out.String(), repairHint) {
		t.Errorf("output:\n%s", out)
	}
}

func TestDoctorGolangciChecks(t *testing.T) {
	cases := map[string]string{
		"golangci-lint has version 1.64.8 built with go1.27.4": "v2 required",
		"golangci-lint has version v2.6.2 built with go1.24.1": "older than go 1.27.0",
		"something else": "unrecognized version output",
	}
	for banner, want := range cases {
		f := newFake()
		f.have["golangci-lint"] = banner
		f.installed["golangci-lint"] = banner // reinstalling does not help: shadowed
		a, out := newApp(t, f)
		if code := a.doctor(context.Background(), true); code != 1 {
			t.Fatalf("%q: code %d", banner, code)
		}
		if !strings.Contains(out.String(), want) {
			t.Errorf("%q: missing %q in\n%s", banner, want, out)
		}
	}
}

func TestDoctorShadowedToolHint(t *testing.T) {
	f := newFake()
	f.have["golangci-lint"] = "golangci-lint has version 1.64.8"
	f.resp[goEnvLine] = "go1.27.4\n" + filepath.FromSlash("/home/u/go/bin") + "\n/gopath\nauto\n"
	a, out := newApp(t, f)
	a.getenv = func(string) string { return filepath.FromSlash("/home/u/go/bin") }
	a.doctor(context.Background(), false)
	if !strings.Contains(out.String(), "comes first on PATH") {
		t.Errorf("no shadow hint:\n%s", out)
	}
}

func TestDoctorToolDoesNotRun(t *testing.T) {
	f := newFake()
	f.fail["govulncheck -version"] = errors.New("exit status 2")
	f.fail["go install golang.org/x/vuln/cmd/govulncheck@latest"] = errors.New("network down")
	a, out := newApp(t, f)
	if code := a.doctor(context.Background(), true); code != 1 {
		t.Fatalf("code %d", code)
	}
	if !strings.Contains(out.String(), "install failed: network down") {
		t.Errorf("output:\n%s", out)
	}
}

func TestDoctorPathAndFlutterWarnings(t *testing.T) {
	f := newFake()
	delete(f.have, "flutter")
	a, out := newApp(t, f)
	a.getenv = func(string) string { return "" }
	if code := a.doctor(context.Background(), false); code != 1 {
		t.Fatalf("code %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "is not on PATH") || !strings.Contains(s, "export PATH") {
		t.Errorf("path hint missing:\n%s", s)
	}
	if !strings.Contains(s, "needed only for the desktop app") {
		t.Errorf("flutter warning missing:\n%s", s)
	}
	out.Reset()
	a.goos = "windows"
	a.checkPath(fakeBin)
	if r := a.checkPath(fakeBin); !strings.Contains(r.hint, "Environment Variables") {
		t.Errorf("windows hint: %q", r.hint)
	}
}

func TestFlutterChannelAndFailure(t *testing.T) {
	f := newFake()
	f.have["flutter"] = "Flutter 3.39.0-1.0.pre • channel beta"
	a, _ := newApp(t, f)
	if r := a.checkFlutter(context.Background()); r.st != warn || !strings.Contains(r.detail, "not on the stable") {
		t.Errorf("beta: %+v", r)
	}
	f.fail["flutter --version"] = errors.New("boom")
	if r := a.checkFlutter(context.Background()); r.st != warn || !strings.Contains(r.detail, "does not run") {
		t.Errorf("broken: %+v", r)
	}
}

func TestGitMissing(t *testing.T) {
	f := newFake()
	delete(f.have, "git")
	a, out := newApp(t, f)
	if code := a.doctor(context.Background(), true); code != 1 {
		t.Fatalf("code %d", code)
	}
	if f.ran("git config core.hooksPath scripts/hooks") || !strings.Contains(out.String(), "git-scm.com") {
		t.Errorf("output:\n%s calls %v", out, f.calls)
	}
}

func TestGoProblems(t *testing.T) {
	f := newFake()
	delete(f.have, "go")
	a, out := newApp(t, f)
	code := run(context.Background(), a, []string{"install"})
	if code != 1 || !strings.Contains(out.String(), "go.dev/dl") {
		t.Fatalf("missing go: code %d\n%s", code, out)
	}

	f = newFake()
	f.resp[goEnvLine] = "go1.26.2\n\n/gopath\nlocal\n"
	a, out = newApp(t, f)
	if code := run(context.Background(), a, []string{"update"}); code != 1 {
		t.Fatalf("old go: code %d", code)
	}
	if !strings.Contains(out.String(), "GOTOOLCHAIN=local blocks") {
		t.Errorf("output:\n%s", out)
	}

	f = newFake()
	f.fail[goEnvLine] = errors.New("download failed")
	a, out = newApp(t, f)
	if code := a.doctor(context.Background(), true); code != 1 || !strings.Contains(out.String(), "network") {
		t.Errorf("go env failure: code %d\n%s", code, out)
	}
}

func TestGoBinFromGopath(t *testing.T) {
	f := newFake()
	f.resp[goEnvLine] = "go1.27.4\n\n" + filepath.FromSlash("/gp") + string(filepath.ListSeparator) + "/other\nauto\n"
	a, _ := newApp(t, f)
	gi, res := a.checkGo(context.Background())
	if res.st != ok || gi.bin != filepath.Join(filepath.FromSlash("/gp"), "bin") {
		t.Errorf("bin %q res %+v", gi.bin, res)
	}
	f.resp[goEnvLine] = "go1.27.4\n\n\nauto\n"
	if _, res := a.checkGo(context.Background()); res.st != fail {
		t.Errorf("no GOPATH: %+v", res)
	}
}

func TestInstall(t *testing.T) {
	f := newFake()
	a, out := newApp(t, f)
	if code := run(context.Background(), a, []string{"install"}); code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, tl := range goTools {
		if !f.ran("go install " + tl.pkg + "@latest") {
			t.Errorf("%s not installed", tl.name)
		}
	}
	if !f.ran("git config core.hooksPath scripts/hooks") || !f.ran("flutter pub get") {
		t.Errorf("calls %v", f.calls)
	}
}

func TestInstallKeepsGoingOnErrors(t *testing.T) {
	f := newFake()
	f.fail["go install github.com/go-task/task/v3/cmd/task@latest"] = errors.New("proxy error")
	f.fail["git config core.hooksPath scripts/hooks"] = errors.New("locked")
	f.fail["flutter pub get"] = errors.New("offline")
	a, out := newApp(t, f)
	run(context.Background(), a, []string{"install"})
	for _, want := range []string{"proxy error", "locked", "offline"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestUpdate(t *testing.T) {
	f := newFake()
	a, out := newApp(t, f)
	if code := run(context.Background(), a, []string{"update"}); code != 0 {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, want := range []string{"go get toolchain@latest", "go mod tidy", "flutter upgrade"} {
		if !f.ran(want) {
			t.Errorf("%q not run: %v", want, f.calls)
		}
	}

	f = newFake()
	f.fail["go get toolchain@latest"] = errors.New("offline")
	a, out = newApp(t, f)
	if code := a.update(context.Background()); code != 1 || f.ran("go mod tidy") {
		t.Errorf("code %d calls %v\n%s", code, f.calls, out)
	}

	f = newFake()
	f.fail["flutter upgrade"] = errors.New("dirty checkout")
	a, out = newApp(t, f)
	if code := a.update(context.Background()); code != 0 || !strings.Contains(out.String(), "dirty checkout") {
		t.Errorf("flutter failure: code %d\n%s", code, out)
	}
}

func TestUpdateNewToolchainFails(t *testing.T) {
	f := newFake()
	a, out := newApp(t, f)
	// After go mod tidy, go env fails (new toolchain download blocked).
	r := &switchingFake{fake: f, after: "go mod tidy"}
	a.r = r
	if code := a.update(context.Background()); code != 1 || !strings.Contains(out.String(), "go env failed") {
		t.Errorf("code %d\n%s", code, out)
	}
}

type switchingFake struct {
	*fake
	after string
}

func (s *switchingFake) run(ctx context.Context, c cmd) (string, error) {
	out, err := s.fake.run(ctx, c)
	if s.ran(s.after) {
		s.fail[goEnvLine] = errors.New("toolchain not available")
	}
	return out, err
}

func TestUsageAndUnknown(t *testing.T) {
	a, out := newApp(t, newFake())
	if code := run(context.Background(), a, nil); code != 2 {
		t.Errorf("no args: %d", code)
	}
	if code := run(context.Background(), a, []string{"help"}); code != 0 {
		t.Errorf("help: %d", code)
	}
	code := run(context.Background(), a, []string{"bogus"})
	if code != 2 || !strings.Contains(out.String(), "unknown") {
		t.Errorf("bogus: %d", code)
	}
	if code := run(context.Background(), a, []string{"doctor", "-nope"}); code != 2 {
		t.Errorf("bad flag: %d", code)
	}
}

func TestReadGoMod(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := readGoMod(dir); err == nil {
		t.Error("missing go.mod accepted")
	}
	if _, _, err := readGoMod(filepath.Join(dir, "nope")); err == nil {
		t.Error("missing dir accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readGoMod(dir); err == nil {
		t.Error("no go directive accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	g, tc, err := readGoMod(dir)
	if err != nil || g != "1.27.0" || tc != "" {
		t.Errorf("got %q %q %v", g, tc, err)
	}
	a, _ := newApp(t, newFake())
	a.root = dir
	if _, res := a.checkGo(context.Background()); res.st != ok || !strings.Contains(res.detail, "toolchain go1.27.0") {
		t.Errorf("no toolchain line: %+v", res)
	}
	a.root = t.TempDir()
	if _, res := a.checkGo(context.Background()); res.st != fail {
		t.Errorf("no go.mod: %+v", res)
	}
}

func TestCompareGo(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"go1.27.4", "go1.27.4", 0},
		{"go1.27.4", "go1.27.10", -1},
		{"go1.28", "go1.27.4", 1},
		{"go1.27rc1", "1.27.0", 0},
		{"go1.26.9", "1.27.0", -1},
	}
	for _, c := range cases {
		if got := compareGo(c.a, c.b); got != c.want {
			t.Errorf("compareGo(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSamePath(t *testing.T) {
	if !samePath(`C:\Go\Bin\`, `c:\go\bin`, "windows") || samePath("/a/B", "/a/b", "linux") {
		t.Error("samePath")
	}
	if samePath("", "/a", "linux") {
		t.Error("samePath")
	}
}

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(goModText), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := findRoot(sub); err != nil || got != root {
		t.Errorf("got %q %v", got, err)
	}
	if _, err := findRoot(filepath.VolumeName(root) + string(filepath.Separator)); err == nil {
		t.Skip("a go.mod exists at the filesystem root")
	}
}

func TestOSRunner(t *testing.T) {
	out := &bytes.Buffer{}
	r := osRunner{out: out}
	goPath, err := r.lookPath("go")
	if err != nil {
		t.Skip("go not on PATH")
	}
	if _, err := r.lookPath("tessivo-no-such-tool"); err == nil {
		t.Error("found a missing tool")
	}
	got, err := r.run(context.Background(), cmd{name: goPath, args: []string{"env", "GOOS"}, stream: true})
	if err != nil || strings.TrimSpace(got) == "" || out.Len() == 0 {
		t.Errorf("got %q %v, streamed %q", got, err, out)
	}
	_, err = r.run(context.Background(), cmd{name: goPath, args: []string{"no-such-command"}, combine: true})
	if err == nil {
		t.Error("bad command succeeded")
	}
	_, err = r.run(context.Background(), cmd{name: goPath, args: []string{"no-such-command"}})
	if err == nil || !strings.Contains(err.Error(), "no-such-command") {
		t.Errorf("error lacks stderr: %v", err)
	}
}

func TestCappedAndLines(t *testing.T) {
	c := &capped{max: 4}
	if n, err := c.Write([]byte("abcdef")); n != 6 || err != nil || c.String() != "abcd" {
		t.Errorf("capped: %d %v %q", n, err, c.String())
	}
	if firstLine("\n  one \ntwo\n") != "one" || lastLine("one\ntwo\n\n") != "two" || firstLine("") != "" {
		t.Error("lines")
	}
	if long := firstLine(strings.Repeat("x", 300)); len(long) > 210 {
		t.Errorf("not clipped: %d", len(long))
	}
	if lastLine("") != "" {
		t.Error("empty lastLine")
	}
	if fail.String() != "FAIL" || warn.String() != "warn" || ok.String() != "ok" || status(9).String() != "status(9)" {
		t.Error("status strings")
	}
}

func TestRaceDetector(t *testing.T) {
	f := newFake()
	a, _ := newApp(t, f)
	if r := a.checkRace(context.Background()); r.st != ok {
		t.Errorf("cgo on: %+v", r)
	}
	f.resp["go env CGO_ENABLED"] = "0\n"
	for goos, want := range map[string]string{"windows": "winget", "darwin": "xcode-select", "linux": "apt"} {
		a.goos = goos
		if r := a.checkRace(context.Background()); r.st != warn || !strings.Contains(r.hint, want) {
			t.Errorf("%s: %+v", goos, r)
		}
	}
}
