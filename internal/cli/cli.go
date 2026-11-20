// Package cli implements the tess command-line interface on top of the core.
// cmd/tess only calls Main; everything here is testable with injected I/O.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// Version is set at build time with -ldflags "-X .../internal/cli.Version=v1.2.3".
var Version = "dev"

// Exit codes.
const (
	ExitOK            = 0
	ExitError         = 1
	ExitUsage         = 2
	ExitInvalidOutput = 3
)

// Env carries everything the CLI touches, so tests can replace it.
type Env struct {
	Stdin      io.Reader
	StdinIsTTY bool
	Stdout     io.Writer
	Stderr     io.Writer
	ConfigPath string
	Secrets    secrets.Resolver
	// NewModel builds the model for an alias; defaults to an llm.Router.
	NewModel func(f config.File, alias config.Alias, sec secrets.Resolver) (executor.Model, error)
	// ListModels lists a provider's models; defaults to the llm client.
	ListModels func(ctx context.Context, p config.Provider, sec secrets.Resolver) ([]llm.Model, error)
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

const usage = `tess — analyze, summarize, document and report with self-hosted models

Usage:
  tess new <dir> --type analyze|summarize|document|report [--name NAME]
  tess context add <notebook> <path>...      attach files (copied) or folders (referenced)
  tess context list <notebook>
  tess context enable|disable <notebook> <id-or-label>
  tess run [<notebook>] [flags]             run a notebook, or pipe text: cat x | tess run --type summarize
  tess outputs <notebook>                   list saved outputs, newest first
  tess models [--provider NAME]             list models from configured providers
  tess worktypes                            list work types
  tess audit verify|show [<notebook-or-file>] [--last N]   check or read the tamper-evident audit log
  tess serve [--network unix|tcp] [--addr A]   run the local API for the desktop app
  tess config init|show|path
  tess version

Run 'tess run -h' for run flags.`

// Main runs the CLI and returns the process exit code.
func Main(ctx context.Context, args []string, env Env) int {
	env = withDefaults(env)
	err := dispatch(ctx, args, env)
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, flag.ErrHelp):
		return ExitOK
	case errors.As(err, new(usageError)):
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n\n%s\n", err, usage)
		return ExitUsage
	case errors.Is(err, llm.ErrInvalidOutput):
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		return ExitInvalidOutput
	default:
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		return ExitError
	}
}

func withDefaults(env Env) Env {
	if env.Secrets == nil {
		env.Secrets = secrets.Env{}
	}
	if env.NewModel == nil {
		env.NewModel = func(f config.File, a config.Alias, s secrets.Resolver) (executor.Model, error) {
			return llm.NewRouter(f, a, s)
		}
	}
	if env.ListModels == nil {
		env.ListModels = listModels
	}
	return env
}

func dispatch(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("missing command")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "version", "--version":
		_, err := fmt.Fprintf(env.Stdout, "tess %s\n", Version)
		return wrapWrite(err)
	case "help", "-h", "--help":
		_, err := fmt.Fprintln(env.Stdout, usage)
		return wrapWrite(err)
	case "new":
		return cmdNew(rest, env)
	case "context":
		return cmdContext(rest, env)
	case "run":
		return cmdRun(ctx, rest, env)
	case "outputs":
		return cmdOutputs(rest, env)
	case "models":
		return cmdModels(ctx, rest, env)
	case "worktypes":
		return cmdWorkTypes(env)
	case "config":
		return cmdConfig(rest, env)
	case "audit":
		return cmdAudit(rest, env)
	case "serve":
		return cmdServe(ctx, rest, env)
	default:
		return usagef("unknown command %q", cmd)
	}
}

func sortStrings(s []string) { sort.Strings(s) }

func wrapWrite(err error) error {
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func newFlags(name string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet("tess "+name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	return fs
}

// parse handles "flags anywhere" by moving positional args after flags.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(strings.SplitN(a, "=", 2)[0], "-")
			if f := fs.Lookup(name); f != nil && !strings.Contains(a, "=") && !isBool(f) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	if err := fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, flag.ErrHelp
		}
		return nil, usageError{err.Error()}
	}
	return pos, nil
}

func isBool(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// multiFlag collects repeated --var k=v flags.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func parseVars(vs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, v := range vs {
		k, val, ok := strings.Cut(v, "=")
		if !ok || k == "" {
			return nil, usagef("--var %q: want name=value", v)
		}
		out[k] = val
	}
	return out, nil
}
