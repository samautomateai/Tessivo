// Package testutil holds OS-independent helpers for tests: permission checks
// that understand Windows, paths that cannot be written on any OS, and a
// helper process that replaces POSIX tools such as sh, true and cat.
//
// It is imported only from _test.go files.
package testutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// AssertPerm fails unless path exists and, where the OS has Unix permission
// bits, has exactly want. Windows controls access with ACLs and Go reports
// only the read-only attribute (0666 or 0444), so there only existence is
// checked.
func AssertPerm(t testing.TB, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s: permissions %v, want %v", path, got, want)
	}
}

// Unwritable returns a path under which nothing can be created on any OS,
// without root-dependent tricks such as /proc or chmod: its parent is a
// regular file, so creating the path or any of its directories fails.
func Unwritable(t testing.TB, elem ...string) string {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(append([]string{blocker}, elem...)...)
}

// helperArg marks a test binary started by HelperCommand.
const helperArg = "tessivo-test-helper"

// HelperCommand returns a program and arguments that run the current test
// binary as a small helper instead of a POSIX tool. The package's TestMain
// must call RunHelper first. Commands:
//
//	exit N          exit with status N (true is "exit 0", false is "exit 1")
//	cat             copy stdin to stdout
//	fail TEXT N     write TEXT to stderr and exit with status N
func HelperCommand(t testing.TB, args ...string) (string, []string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe, append([]string{helperArg}, args...)
}

// RunHelper turns the process into the helper when HelperCommand started it,
// and exits; otherwise it returns at once. Call it first in TestMain.
func RunHelper() {
	if len(os.Args) < 2 || os.Args[1] != helperArg {
		return
	}
	os.Exit(helper(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
}

func helper(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	switch cmd := strings.Join(args, "\x00"); {
	case cmd == "cat":
		if _, err := io.Copy(stdout, stdin); err != nil {
			return 1
		}
		return 0
	case strings.HasPrefix(cmd, "exit\x00") && len(args) == 2:
		return atoi(strings.TrimPrefix(cmd, "exit\x00"))
	case strings.HasPrefix(cmd, "fail\x00") && len(args) == 3:
		text, code, _ := strings.Cut(strings.TrimPrefix(cmd, "fail\x00"), "\x00")
		_, _ = fmt.Fprintln(stderr, text)
		return atoi(code)
	default:
		_, _ = fmt.Fprintf(stderr, "testutil helper: bad command %q\n", args)
		return 125
	}
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 125
	}
	return n
}
