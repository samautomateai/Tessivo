package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type status int

const (
	ok status = iota
	warn
	fail
)

func (s status) String() string {
	switch s {
	case ok:
		return "ok"
	case warn:
		return "warn"
	case fail:
		return "FAIL"
	}
	return "status(" + strconv.Itoa(int(s)) + ")"
}

type result struct {
	name   string
	st     status
	detail string
	hint   string
}

// tool is a Go program the quality gates call by name.
type tool struct {
	name    string
	pkg     string   // go install path
	version []string // arguments that make it print its version
	verify  func(out string, gi goInfo) error
}

var goTools = []tool{
	{name: "task", pkg: "github.com/go-task/task/v3/cmd/task", version: []string{versionFlag}},
	{
		name: "golangci-lint", pkg: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		version: []string{"version"}, verify: verifyGolangci,
	},
	{name: "govulncheck", pkg: "golang.org/x/vuln/cmd/govulncheck", version: []string{"-version"}},
	{name: "gosec", pkg: "github.com/securego/gosec/v2/cmd/gosec", version: []string{"-version"}},
	// osv-scanner checks the desktop app's pubspec.lock; govulncheck covers only Go.
	{name: "osv-scanner", pkg: "github.com/google/osv-scanner/v2/cmd/osv-scanner", version: []string{versionFlag}},
}

const (
	hooksDir    = "scripts/hooks"
	flutter     = "flutter"
	versionFlag = "--version"
	windows     = "windows"
	repairHint  = "run: task doctor (or: go run ./tools/devtools doctor)"
)

type app struct {
	r       runner
	out     io.Writer
	root    string
	getenv  func(string) string
	goos    string
	timeout time.Duration
}

func (a *app) say(format string, args ...any) {
	_, _ = fmt.Fprintf(a.out, format+"\n", args...)
}

func (a *app) exec(ctx context.Context, c cmd) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.r.run(ctx, c)
}

// goInfo is what the Go installation reports from inside the repository.
type goInfo struct {
	version     string // GOVERSION of the toolchain go.mod selects
	bin         string // where go install puts programs
	goLine      string // go directive, e.g. 1.27.0
	toolchain   string // toolchain directive, e.g. go1.27.4 (may be empty)
	gotoolchain string // GOTOOLCHAIN setting
}

func (a *app) checkGo(ctx context.Context) (goInfo, result) {
	res := result{name: "go", st: fail}
	var gi goInfo
	var err error
	if gi.goLine, gi.toolchain, err = readGoMod(a.root); err != nil {
		res.detail = err.Error()
		return gi, res
	}
	if _, err := a.r.lookPath("go"); err != nil {
		res.detail, res.hint = "not installed", "install Go from https://go.dev/dl, then run this again"
		return gi, res
	}
	env := cmd{dir: a.root, name: "go", args: []string{"env", "GOVERSION", "GOBIN", "GOPATH", "GOTOOLCHAIN"}}
	out, err := a.exec(ctx, env)
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	if err != nil || len(lines) < 4 {
		res.detail = fmt.Sprintf("go env failed: %v", err)
		res.hint = "Go downloads the toolchain pinned in go.mod; check network access"
		return gi, res
	}
	gi.version, gi.bin = strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	gi.gotoolchain = strings.TrimSpace(lines[3])
	if gi.bin == "" {
		gopath := filepath.SplitList(strings.TrimSpace(lines[2]))
		if len(gopath) == 0 {
			res.detail = "neither GOBIN nor GOPATH is set"
			return gi, res
		}
		gi.bin = filepath.Join(gopath[0], "bin")
	}
	want := gi.toolchain
	if want == "" {
		want = "go" + gi.goLine
	}
	if compareGo(gi.version, want) < 0 {
		res.detail = fmt.Sprintf("%s is older than %s pinned in go.mod", gi.version, want)
		res.hint = fmt.Sprintf("GOTOOLCHAIN=%s blocks the switch; run: go env -w GOTOOLCHAIN=auto", gi.gotoolchain)
		return gi, res
	}
	res.st, res.detail = ok, fmt.Sprintf("%s (go.mod: go %s, toolchain %s)", gi.version, gi.goLine, want)
	return gi, res
}

func (a *app) checkPath(bin string) result {
	res := result{name: "PATH"}
	for _, d := range filepath.SplitList(a.getenv("PATH")) {
		if samePath(d, bin, a.goos) {
			res.st, res.detail = ok, "includes "+bin
			return res
		}
	}
	res.st, res.detail = fail, bin+" is not on PATH, so Task cannot find the tools installed there"
	if a.goos == windows {
		res.hint = "add " + bin + " to your user Path (Settings > System > About > Advanced > Environment Variables)"
	} else {
		res.hint = `add to your shell profile: export PATH="$PATH:` + bin + `"`
	}
	return res
}

func samePath(a, b, goos string) bool {
	if a == "" || b == "" {
		return false
	}
	// Compare by the target OS's rules, not the host's: filepath.Clean would
	// treat backslashes differently depending on where this runs.
	if goos == windows {
		a, b = strings.ReplaceAll(a, `\`, "/"), strings.ReplaceAll(b, `\`, "/")
		return strings.EqualFold(path.Clean(a), path.Clean(b))
	}
	return path.Clean(a) == path.Clean(b)
}

// find returns the copy of a tool the shell would run, else the one in bin.
func (a *app) find(name, bin string) string {
	if p, err := a.r.lookPath(name); err == nil {
		return p
	}
	if p, err := a.r.lookPath(filepath.Join(bin, name)); err == nil {
		return p
	}
	return ""
}

func (a *app) checkTool(ctx context.Context, gi goInfo, t tool) result {
	res := result{name: t.name, st: fail, hint: repairHint}
	p := a.find(t.name, gi.bin)
	if p == "" {
		res.detail = "not installed"
		return res
	}
	out, err := a.exec(ctx, cmd{dir: a.root, name: p, args: t.version, combine: true})
	if err != nil {
		res.detail = "does not run: " + firstLine(err.Error())
		return res
	}
	if t.verify != nil {
		if err := t.verify(out, gi); err != nil {
			res.detail = err.Error()
			if !samePath(filepath.Dir(p), gi.bin, a.goos) {
				res.hint = fmt.Sprintf("%s comes first on PATH; remove it or put %s first", p, gi.bin)
			}
			return res
		}
	}
	banner, _, _ := strings.Cut(firstLine(out), " from ") // drop build provenance
	res.st, res.detail, res.hint = ok, banner, ""
	return res
}

var (
	golangciVersion = regexp.MustCompile(`version v?(\d+)\.\d+`)
	golangciBuiltBy = regexp.MustCompile(`built with (go\d+\.\d+(?:\.\d+)?)`)
)

// verifyGolangci requires golangci-lint v2 built with a Go at least as new as
// the go directive; an older build cannot load the code.
func verifyGolangci(out string, gi goInfo) error {
	m := golangciVersion.FindStringSubmatch(out)
	if m == nil {
		return fmt.Errorf("unrecognized version output: %s", firstLine(out))
	}
	if m[1] != "2" {
		return fmt.Errorf("version %s.x found, v2 required", m[1])
	}
	if b := golangciBuiltBy.FindStringSubmatch(out); b != nil && compareGo(b[1], "go"+gi.goLine) < 0 {
		return fmt.Errorf("built with %s, older than go %s in go.mod", b[1], gi.goLine)
	}
	return nil
}

func (a *app) checkHooks(ctx context.Context) result {
	res := result{name: "git hooks", st: fail}
	if _, err := a.r.lookPath("git"); err != nil {
		res.detail, res.hint = "git not installed", "install Git from https://git-scm.com/downloads"
		return res
	}
	// git exits 1 when the key is unset; the empty output says the same.
	out, _ := a.exec(ctx, cmd{dir: a.root, name: "git", args: []string{"config", "--get", "core.hooksPath"}})
	if strings.TrimSpace(out) == hooksDir {
		res.st, res.detail = ok, "core.hooksPath = "+hooksDir
		return res
	}
	res.detail, res.hint = "core.hooksPath is not "+hooksDir, "run: task hooks:install"
	return res
}

func (a *app) setHooks(ctx context.Context) error {
	_, err := a.exec(ctx, cmd{dir: a.root, name: "git", args: []string{"config", "core.hooksPath", hooksDir}})
	return err
}

// checkRace only warns: the race detector needs cgo, which Go turns off when no
// C compiler is found. CI (Linux) always runs it.
func (a *app) checkRace(ctx context.Context) result {
	res := result{name: "race detector", st: warn}
	out, err := a.exec(ctx, cmd{dir: a.root, name: "go", args: []string{"env", "CGO_ENABLED"}})
	if err == nil && strings.TrimSpace(out) == "1" {
		res.st, res.detail = ok, "cgo enabled; go test -race works"
		return res
	}
	res.detail = "cgo is off (no C compiler found), so tests run without -race locally"
	switch a.goos {
	case windows:
		res.hint = "install gcc (e.g. winget install BrechtSanders.WinLibs.POSIX.UCRT), open a new shell, " +
			"then: go env -w CGO_ENABLED=1"
	case "darwin":
		res.hint = "run: xcode-select --install"
	default:
		res.hint = "install gcc (e.g. apt install gcc)"
	}
	return res
}

// checkFlutter only warns: Flutter is needed for the desktop app alone.
func (a *app) checkFlutter(ctx context.Context) result {
	res := result{name: flutter, st: warn}
	if _, err := a.r.lookPath(flutter); err != nil {
		res.detail = "not installed (needed only for the desktop app)"
		res.hint = strings.Join(flutterInstallSteps(a.goos), "\n")
		return res
	}
	out, err := a.exec(ctx, cmd{dir: a.root, name: flutter, args: []string{versionFlag}, combine: true})
	if err != nil {
		res.detail = "does not run: " + firstLine(err.Error())
		return res
	}
	if !strings.Contains(out, "channel stable") {
		res.detail = "not on the stable channel: " + firstLine(out)
		res.hint = "run: flutter channel stable && task update"
		return res
	}
	res.st, res.detail = ok, firstLine(out)
	return res
}

// readGoMod returns the go and toolchain directives of root/go.mod.
func readGoMod(root string) (goLine, toolchain string, err error) {
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", "", fmt.Errorf("open repository: %w", err)
	}
	defer func() { _ = r.Close() }()
	f, err := r.Open("go.mod")
	if err != nil {
		return "", "", fmt.Errorf("read go.mod: %w", err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(io.LimitReader(f, maxOutput))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "go":
			goLine = fields[1]
		case "toolchain":
			toolchain = fields[1]
		}
	}
	if err := sc.Err(); err != nil {
		return "", "", fmt.Errorf("read go.mod: %w", err)
	}
	if goLine == "" {
		return "", "", errors.New("go.mod has no go directive")
	}
	return goLine, toolchain, nil
}

// compareGo orders Go versions such as go1.27.4, go1.27 and 1.27.0.
func compareGo(a, b string) int {
	pa, pb := goParts(a), goParts(b)
	for i := range pa {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func goParts(v string) [3]int {
	var parts [3]int
	for i, s := range strings.SplitN(strings.TrimPrefix(v, "go"), ".", 3) {
		end := 0
		for end < len(s) && s[end] >= '0' && s[end] <= '9' {
			end++
		}
		parts[i], _ = strconv.Atoi(s[:end])
	}
	return parts
}
