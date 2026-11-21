// Command fuzzall runs every Go fuzz target in the given packages for a short
// time. It finds targets with `go test -list`, so it needs nothing but Go and
// works the same on Windows, macOS and Linux.
//
// Usage:
//
//	fuzzall [-fuzztime 20s] [packages]   # packages default to ./...
//
// It runs every target even after a failure, then exits 1 if any failed.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
)

// maxTargets bounds what one run will fuzz.
const maxTargets = 500

type target struct{ pkg, name string }

// goRunner runs `go` with args; with stream it also shows the output live.
type goRunner func(ctx context.Context, stream bool, args ...string) (string, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:], os.Stdout, execGo(os.Stdout))
	stop()
	os.Exit(code)
}

func execGo(out io.Writer) goRunner {
	return func(ctx context.Context, stream bool, args ...string) (string, error) {
		var buf strings.Builder
		c := exec.CommandContext(ctx, "go", args...)
		c.Stdout, c.Stderr = &buf, &buf
		if stream {
			c.Stdout, c.Stderr = io.MultiWriter(&buf, out), io.MultiWriter(&buf, out)
		}
		if err := c.Run(); err != nil {
			return buf.String(), fmt.Errorf("go %s: %w", strings.Join(args, " "), err)
		}
		return buf.String(), nil
	}
}

func run(ctx context.Context, args []string, out io.Writer, goCmd goRunner) int {
	fl := flag.NewFlagSet("fuzzall", flag.ContinueOnError)
	fl.SetOutput(out)
	fuzztime := fl.String("fuzztime", "20s", "time (or NNNx iterations) to fuzz each target")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	pkgs := fl.Args()
	if len(pkgs) == 0 {
		pkgs = []string{"./..."}
	}
	listing, err := goCmd(ctx, false, append([]string{"test", "-list", "^Fuzz"}, pkgs...)...)
	if err != nil {
		_, _ = fmt.Fprintf(out, "fuzzall: %v\n%s", err, listing)
		return 2
	}
	targets, err := parseList(listing)
	if err != nil {
		_, _ = fmt.Fprintln(out, "fuzzall:", err)
		return 2
	}
	var failed []string
	for _, t := range targets {
		_, _ = fmt.Fprintf(out, "fuzzing %s %s for %s\n", t.pkg, t.name, *fuzztime)
		_, err := goCmd(ctx, true, "test", "-run", "^$", "-fuzz", "^"+t.name+"$", "-fuzztime", *fuzztime, t.pkg)
		if err != nil {
			failed = append(failed, t.pkg+" "+t.name)
		}
	}
	_, _ = fmt.Fprintf(out, "fuzzall: %d target(s), %d failed\n", len(targets), len(failed))
	for _, f := range failed {
		_, _ = fmt.Fprintln(out, "  FAIL", f)
	}
	if len(failed) > 0 {
		return 1
	}
	return 0
}

var fuzzName = regexp.MustCompile(`^Fuzz[A-Za-z0-9_]*$`)

// parseList reads `go test -list ^Fuzz` output: each package prints its
// target names, then a line "ok  <import path>  <time>".
func parseList(listing string) ([]target, error) {
	var targets []target
	var pending []string
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case fuzzName.MatchString(line):
			pending = append(pending, line)
		case strings.HasPrefix(line, "ok"):
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return nil, fmt.Errorf("unexpected line %q", line)
			}
			for _, name := range pending {
				targets = append(targets, target{pkg: fields[1], name: name})
			}
			pending = nil
		}
		if len(targets) > maxTargets {
			return nil, fmt.Errorf("more than %d fuzz targets", maxTargets)
		}
	}
	if len(pending) > 0 {
		return nil, fmt.Errorf("targets %v without a package line", pending)
	}
	return targets, nil
}
