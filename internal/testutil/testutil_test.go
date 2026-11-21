package testutil

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	RunHelper()
	os.Exit(m.Run())
}

func TestAssertPerm(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(p, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	AssertPerm(t, p, 0o600)
}

func TestUnwritable(t *testing.T) {
	p := Unwritable(t, "sub", "file.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err == nil {
		t.Fatal("created a directory under a file")
	}
	if err := os.WriteFile(p, []byte("x"), 0o600); err == nil {
		t.Fatal("wrote under a file")
	}
}

func TestHelperProcess(t *testing.T) {
	run := func(stdin string, args ...string) (string, string, int) {
		name, argv := HelperCommand(t, args...)
		c := exec.Command(name, argv...)
		c.Stdin = strings.NewReader(stdin)
		var out, errb bytes.Buffer
		c.Stdout, c.Stderr = &out, &errb
		err := c.Run()
		var ee *exec.ExitError
		if err != nil && !errors.As(err, &ee) {
			t.Fatal(err)
		}
		return out.String(), errb.String(), c.ProcessState.ExitCode()
	}
	if _, _, code := run("", "exit", "7"); code != 7 {
		t.Errorf("exit: %d", code)
	}
	if out, _, code := run("piped", "cat"); out != "piped" || code != 0 {
		t.Errorf("cat: %q %d", out, code)
	}
	if _, errOut, code := run("", "fail", "nope", "3"); !strings.Contains(errOut, "nope") || code != 3 {
		t.Errorf("fail: %q %d", errOut, code)
	}
}

func TestHelperDirect(t *testing.T) {
	var out, errb bytes.Buffer
	if helper([]string{"bogus"}, nil, &out, &errb) != 125 || !strings.Contains(errb.String(), "bad command") {
		t.Error("bad command")
	}
	if helper([]string{"exit", "x"}, nil, &out, &errb) != 125 {
		t.Error("bad number")
	}
	if helper([]string{"cat"}, failReader{}, &out, &errb) != 1 {
		t.Error("cat read error")
	}
}

type failReader struct{}

func (failReader) Read([]byte) (int, error) { return 0, errors.New("broken") }
