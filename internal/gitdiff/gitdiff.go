// Package gitdiff reads a repository's changes against a base branch, for
// use as a source: committed changes since the base, then what is not yet
// committed. It runs git directly (no shell) with fixed arguments, a time
// limit and an output cap, and turns off the repository settings that could
// run programs (fsmonitor, hooks, external diff tools, textconv).
package gitdiff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Limits.
const (
	MaxBytes = 2 << 20
	timeout  = 30 * time.Second
)

var refPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

// ErrNotRepo means the folder is not a git repository.
var ErrNotRepo = errors.New("not a git repository")

// CheckBase reports whether base is a branch, tag or commit name git may use.
func CheckBase(base string) error {
	if !refPattern.MatchString(base) || strings.Contains(base, "..") {
		return fmt.Errorf("base %q: want a branch, tag or commit name", base)
	}
	return nil
}

// Diff returns the changes in repo since base, then the uncommitted ones.
func Diff(ctx context.Context, repo, base string) (string, error) {
	if err := CheckBase(base); err != nil {
		return "", err
	}
	if !filepath.IsAbs(repo) {
		return "", fmt.Errorf("repository %s: want an absolute path", repo)
	}
	if info, err := os.Stat(repo); err != nil || !info.IsDir() {
		return "", fmt.Errorf("repository %s: not a folder", repo)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if _, err := run(ctx, repo, "rev-parse", "--git-dir"); err != nil {
		return "", fmt.Errorf("%s: %w", repo, ErrNotRepo)
	}
	committed, err := run(ctx, repo, "diff", "--no-color", "--no-ext-diff", "--no-textconv", base+"...HEAD", "--")
	if err != nil {
		return "", fmt.Errorf("changes since %s: %w", base, err)
	}
	pending, err := run(ctx, repo, "diff", "--no-color", "--no-ext-diff", "--no-textconv", "HEAD", "--")
	if err != nil {
		return "", fmt.Errorf("uncommitted changes: %w", err)
	}
	out := "=== changes since " + base + " ===\n" + committed + "\n=== not yet committed ===\n" + pending
	if len(out) > MaxBytes {
		out = out[:MaxBytes] + "\n[… the rest of the diff was cut]"
	}
	return out, nil
}

// run runs one git command in repo with the risky settings turned off.
func run(ctx context.Context, repo string, args ...string) (string, error) {
	safe := []string{
		"-C", repo, "-c", "core.fsmonitor=false", "-c", "core.hooksPath=",
		"-c", "diff.external=", "-c", "core.pager=cat", "--no-pager",
	}
	// G204: the arguments are fixed and base was checked by CheckBase.
	cmd := exec.CommandContext(ctx, "git", append(safe, args...)...) //nolint:gosec // G204: see above
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EXTERNAL_DIFF=")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limited{w: &stdout, n: MaxBytes + 1}
	cmd.Stderr = &limited{w: &stderr, n: 4 << 10}
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("git is not installed or not on PATH")
		}
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// limited keeps at most n bytes and drops the rest (git keeps writing).
type limited struct {
	w io.Writer
	n int
}

func (l *limited) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		_, _ = l.w.Write(p[:k])
		l.n -= k
	}
	return len(p), nil
}
