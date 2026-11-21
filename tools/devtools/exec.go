package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// maxOutput caps what is kept from one command's output.
const maxOutput = 1 << 20

// cmd is one external command. The program and arguments are fixed by this
// tool, never taken from user text.
type cmd struct {
	dir     string
	env     []string // extra KEY=VALUE entries on top of the current environment
	name    string
	args    []string
	combine bool // return stderr together with stdout (version banners use either)
	stream  bool // also show the output to the user while it runs
}

type runner interface {
	run(ctx context.Context, c cmd) (string, error)
	lookPath(name string) (string, error)
}

type osRunner struct{ out io.Writer }

func (o osRunner) lookPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("look up %s: %w", name, err)
	}
	return p, nil
}

func (o osRunner) run(ctx context.Context, c cmd) (string, error) {
	x := exec.CommandContext(ctx, c.name, c.args...)
	x.Dir = c.dir
	x.Env = append(os.Environ(), c.env...)
	stdout := &capped{max: maxOutput}
	stderr := &capped{max: maxOutput}
	if c.combine {
		stderr = stdout
	}
	x.Stdout, x.Stderr = stdout, stderr
	if c.stream {
		x.Stdout = io.MultiWriter(stdout, o.out)
		x.Stderr = io.MultiWriter(stderr, o.out)
	}
	if err := x.Run(); err != nil {
		detail := lastLine(stderr.String())
		if detail != "" {
			detail = ": " + detail
		}
		return stdout.String(), fmt.Errorf("%s %s: %w%s", c.name, strings.Join(c.args, " "), err, detail)
	}
	return stdout.String(), nil
}

// capped keeps the first max bytes and silently drops the rest, so a noisy
// command can neither exhaust memory nor stop the copy with a short write.
type capped struct {
	buf bytes.Buffer
	max int
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room > 0 {
		c.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (c *capped) String() string { return c.buf.String() }

// firstLine returns the first non-empty line of s, at most 200 bytes.
func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return clip(l)
		}
	}
	return ""
}

// lastLine returns the last non-empty line of s, at most 200 bytes.
func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return clip(l)
		}
	}
	return ""
}

func clip(s string) string {
	const limit = 200
	if len(s) > limit {
		return s[:limit] + "…"
	}
	return s
}
