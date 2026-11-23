package agenttools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/proctree"
)

// RunCommandName is the command tool's name, for rules and approvals.
const RunCommandName = "run_command"

// Limits on one command.
const (
	maxCommandOut = 256 << 10
	// CommandTimeout bounds one command; its whole process tree is stopped then.
	CommandTimeout = 5 * time.Minute
)

// matches reports whether program and args are allowed by r: the same
// program (as written, or the same file after resolving both on PATH), the
// same number of arguments, and every argument equal or matched by "*".
func matches(r config.CommandRule, program string, args []string) bool {
	if len(args) != len(r.Args) || !sameProgram(r.Program, program) {
		return false
	}
	for i, want := range r.Args {
		if want != "*" && want != args[i] {
			return false
		}
	}
	return true
}

func sameProgram(rule, program string) bool {
	if rule == program {
		return true
	}
	a, errA := exec.LookPath(rule)
	b, errB := exec.LookPath(program)
	return errA == nil && errB == nil && filepath.Clean(a) == filepath.Clean(b)
}

type runCommand struct {
	roots *Roots
	allow []config.CommandRule
	env   []string
}

// RunCommandTool runs programs without a shell, in a root, with env as the
// whole environment. Calls matching allow run without asking (Preapproved);
// any other call follows the rules (ask by default).
func RunCommandTool(roots *Roots, allow []config.CommandRule, env []string) Tool {
	return runCommand{roots: roots, allow: allow, env: env}
}

type commandArgs struct {
	Program string   `json:"program"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
}

func (runCommand) Def() llm.ToolDef {
	return llm.ToolDef{
		Name: RunCommandName,
		Description: "Run a program (no shell: no pipes, globs or variables) and get its exit code and output " +
			"(stdout and stderr, at most 256 KiB). dir is a folder inside the context (default: the first one). " +
			"Allow-listed commands run at once; others need the user's approval.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"program":{"type":"string","description":"program name on PATH, or its path"},` +
			`"args":{"type":"array","items":{"type":"string"}},` +
			`"dir":{"type":"string","description":"working folder inside the context"}},` +
			`"required":["program"]}`),
	}
}

// Preapproved reports whether the call matches the allow-list.
func (t runCommand) Preapproved(raw json.RawMessage) bool {
	var a commandArgs
	if decode(raw, &a) != nil {
		return false
	}
	for _, r := range t.allow {
		if matches(r, a.Program, a.Args) {
			return true
		}
	}
	return false
}

func (t runCommand) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var a commandArgs
	if err := decode(raw, &a); err != nil {
		return "", err
	}
	if err := config.CheckCommandRules([]config.CommandRule{{Program: a.Program, Args: a.Args}}); err != nil {
		return "", fmt.Errorf("run_command: %w", err)
	}
	dir, err := t.workDir(a.Dir)
	if err != nil {
		return "", err
	}
	path, err := t.program(a.Program, dir)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()
	out := &capped{limit: maxCommandOut}
	cmd := exec.CommandContext(ctx, path, a.Args...) //nolint:gosec // G204: approved or allow-listed, no shell
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, t.env, out, out
	cmd.WaitDelay = 5 * time.Second
	proctree.Prepare(cmd)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("run_command: %w", err)
	}
	if tree, err := proctree.Attach(cmd); err == nil {
		cmd.Cancel = tree.Kill // a timeout or cancel stops everything it started
		defer func() { _ = tree.Release() }()
	}
	err = cmd.Wait()
	code := 0
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		return out.String() + "\n[stopped: " + ctx.Err().Error() + "]", nil
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		return "", fmt.Errorf("run_command: %w", err)
	}
	return fmt.Sprintf("exit code %d\n%s", code, out.String()), nil
}

// program resolves a bare name on PATH; a relative path such as ./build.sh
// is taken from dir and must stay inside the roots.
func (t runCommand) program(name, dir string) (string, error) {
	if strings.ContainsAny(name, `/\`) && !filepath.IsAbs(name) {
		p, err := t.roots.Resolve(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("run_command: %w", err)
		}
		name = p
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("run_command: %w", err)
	}
	return path, nil
}

// workDir is the call's folder: inside a root, default the first root.
func (t runCommand) workDir(dir string) (string, error) {
	if dir == "" {
		paths := t.roots.Paths()
		if len(paths) == 0 {
			return "", errors.New("run_command: no folder to run in")
		}
		dir = paths[0]
	}
	p, err := t.roots.Resolve(dir)
	if err != nil {
		return "", fmt.Errorf("run_command: %w", err)
	}
	if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("run_command: %s is not a folder", dir)
	}
	return p, nil
}

// capped keeps the first limit bytes and notes that the rest was cut.
type capped struct {
	limit int
	buf   bytes.Buffer
	cut   bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.limit - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(len(p), room)])
	}
	if c.buf.Len() >= c.limit && len(p) > 0 {
		c.cut = true
	}
	return len(p), nil
}

func (c *capped) String() string {
	if c.cut {
		return c.buf.String() + "\n[output cut at 256 KiB]"
	}
	return c.buf.String()
}

// CommandEnv is environ without variables whose names look secret or are
// one of the configured key names.
func CommandEnv(environ, keyNames []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if secretName(name, keyNames) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func secretName(name string, keyNames []string) bool {
	up := strings.ToUpper(name)
	for _, w := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "AUTH"} {
		if strings.Contains(up, w) {
			return true
		}
	}
	for _, k := range keyNames {
		if strings.EqualFold(name, k) {
			return true
		}
	}
	return false
}
