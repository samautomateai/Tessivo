package agenttools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.RunHelper()
	os.Exit(m.Run())
}

// helperCall is a run_command call that runs the test binary as a helper.
func helperCall(t *testing.T, dir string, args ...string) (json.RawMessage, string, []string) {
	t.Helper()
	prog, pargs := testutil.HelperCommand(t, args...)
	b, _ := json.Marshal(map[string]any{"program": prog, "args": pargs, "dir": dir})
	return b, prog, pargs
}

func commandRoots(t *testing.T) (*Roots, string) {
	t.Helper()
	root := t.TempDir()
	r, err := NewRoots(root)
	if err != nil {
		t.Fatal(err)
	}
	return r, root
}

func TestRunCommandRunsWithoutAShell(t *testing.T) {
	r, _ := commandRoots(t)
	args, _, _ := helperCall(t, "", "fail", "boom; echo pwned", "3")
	out, err := RunCommandTool(r, nil, os.Environ()).Run(context.Background(), args)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "exit code 3\n") || !strings.Contains(out, "boom; echo pwned") {
		t.Fatalf("the argument must reach the program as one literal string: %q", out)
	}
}

func TestRunCommandAllowList(t *testing.T) {
	r, root := commandRoots(t)
	args, prog, pargs := helperCall(t, root, "exit", "0")
	rule := config.CommandRule{Program: prog, Args: []string{pargs[0], "exit", "*"}}
	tool := RunCommandTool(r, []config.CommandRule{rule}, nil).(runCommand)
	if !tool.Preapproved(args) {
		t.Fatal("a call matching the allow-list is preapproved")
	}
	other, _, _ := helperCall(t, root, "exit", "0", "extra")
	if tool.Preapproved(other) {
		t.Fatal("an extra argument must not match: * stands for exactly one")
	}
	bad, _ := json.Marshal(map[string]any{"program": prog, "args": []string{pargs[0], "cat", "1"}})
	if tool.Preapproved(bad) || tool.Preapproved(json.RawMessage(`{`)) {
		t.Fatal("other arguments or broken JSON are not preapproved")
	}
}

func TestRunCommandStaysInTheRoots(t *testing.T) {
	r, _ := commandRoots(t)
	tool := RunCommandTool(r, nil, nil)
	args, _, _ := helperCall(t, t.TempDir(), "exit", "0") // a folder outside the roots
	if _, err := tool.Run(context.Background(), args); err == nil {
		t.Fatal("a folder outside the context must be refused")
	}
	rel, _ := json.Marshal(map[string]any{"program": filepath.Join("..", "..", "evil")})
	if _, err := tool.Run(context.Background(), rel); err == nil {
		t.Fatal("a relative program outside the roots must be refused")
	}
	if _, err := tool.Run(context.Background(), json.RawMessage(`{"program":"tessivo-no-such-program"}`)); err == nil {
		t.Fatal("an unknown program must fail")
	}
}

func TestRunCommandCapsOutput(t *testing.T) {
	c := &capped{limit: 8}
	_, _ = c.Write([]byte("0123456789"))
	if c.String() != "01234567\n[output cut at 256 KiB]" {
		t.Fatalf("%q", c.String())
	}
}

func TestCommandEnvDropsSecrets(t *testing.T) {
	got := CommandEnv([]string{
		"PATH=/bin", "GITHUB_TOKEN=x", "AWS_SECRET_ACCESS_KEY=y", "DB_PASSWORD=z", "MY_LITELLM=k", "HOME=/h",
	}, []string{"MY_LITELLM"})
	if strings.Join(got, ",") != "PATH=/bin,HOME=/h" {
		t.Fatalf("%v", got)
	}
}
