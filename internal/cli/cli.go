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
	"github.com/samautomateai/tessivo/internal/keychain"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notify"
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
	ExitPaused        = 4 // an agent run is waiting for approval
)

// Env carries everything the CLI touches, so tests can replace it.
type Env struct {
	Stdin      io.Reader
	StdinIsTTY bool
	Stdout     io.Writer
	Stderr     io.Writer
	ConfigPath string
	Secrets    secrets.Resolver
	Keychain   keychain.Store
	// NewModel builds the model for an alias; defaults to an llm.Router.
	NewModel func(f config.File, alias config.Alias, sec secrets.Resolver) (executor.Model, error)
	// Notifier shows OS notifications; defaults to notify.OS().
	Notifier notify.Notifier
	// ListModels lists a provider's models; defaults to the llm client.
	ListModels func(ctx context.Context, p config.Provider, sec secrets.Resolver) ([]llm.Model, error)
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

const usage = `tess — analyze, summarize, document and report with self-hosted models

Usage:
  tess new <dir> --type analyze|summarize|document|report [--name NAME]
  tess notebook rename|archive|unarchive <notebook> [name]   tess notebook delete --yes <notebook>
  tess prompt list|show|rm <notebook> [name]   tess prompt save <notebook> <name> [--model M] [text|stdin]
  tess usage                                today's token use and limits
  tess search <words> [--limit N]           search notebooks, outputs, prompts and research notes
  tess diagnostics [--out FILE]             a zip for bug reports (no keys, names or content)
  tess config get|set <key> [value]          e.g. tess config set limits.max_tokens_per_day 50000
  tess config export [file]   tess config import <file> [--force]
  tess context add <notebook> <path|url>...  attach files (copied), folders (referenced) or web pages
  tess context add <notebook> --text T|- [--label L] | --git-diff <repo> [--base main]   pasted text, git changes
  tess context list <notebook>
  tess context enable|disable <notebook> <id-or-label>
  tess run [<notebook>] [flags]             run a notebook, or pipe text: cat x | tess run --type summarize
  tess run pending|approve|deny|resume|cancel ...   agent runs waiting for approval (see 'tess run -h')
  tess outputs <notebook>                   list saved outputs, newest first
  tess output ask|suggest|suggestions|apply|dismiss|refine|export|rerun|diff|data|edit <notebook> <output-id> ...
                                            talk to an output, refine, export, run again, compare
  tess mcp add|list|test|remove ...        MCP servers notebooks can attach (see 'tess mcp -h')
  tess research new|list|show|delete|source|ask|note ...   research notebooks: summaries, cited answers
  tess models [--provider NAME]             list models from configured providers
  tess models test <alias|provider/model>   send each model one tiny request: does it answer?
  tess worktypes                            list work types
  tess audit verify|show [<notebook-or-file>] [--last N]   check or read the tamper-evident audit log
  tess serve [--network unix|tcp] [--addr A] [--notify]   run the local API (--notify: OS notifications)
  tess notify test                          check that OS notifications work here
  tess schedule add <notebook> --cron EXPR [--prompt P] [--missed skip|run_once]
  tess schedule list|remove|enable|disable <notebook> [id]   tess schedule next "<cron>"
  tess template list [--type T] | show <id> | use <id> <dir> | save <notebook> --name N
  tess template extend <id> --name N | delete <id>
  tess memory list|add|remove <notebook> [text|number]   pinned facts included in every run
  tess chat [--session ID] [--attach PATH]... [--mcp S,...] [message]   chat (interactive without a message)
  tess chat list|show|rename|pin|archive|delete|memory|repeat|save ...   see 'tess chat -h'
  tess secret set|delete|check <ref>          store API keys in the OS keychain (value read from stdin)
  tess config init|show|path
  tess version

Run 'tess run -h' for run flags. Exit codes: 0 ok, 1 error, 2 usage, 3 invalid output, 4 waiting for approval.`

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
	case errors.As(err, new(pausedError)):
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		return ExitPaused
	case errors.Is(err, llm.ErrInvalidOutput):
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		return ExitInvalidOutput
	default:
		_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		return ExitError
	}
}

func withDefaults(env Env) Env {
	if env.Keychain == nil {
		env.Keychain = keychain.New()
	}
	if env.Secrets == nil {
		env.Secrets = secrets.Chain{secrets.Env{}, keychain.Resolver{Store: env.Keychain}}
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

// Sub-command words shared by several commands.
const (
	subAdd    = "add"
	subList   = "list"
	subShow   = "show"
	subSave   = "save"
	subSet    = "set"
	subExport = "export"
	subTest   = "test"
	subRemove = "remove"
	subDelete = "delete"
)

type handler func(ctx context.Context, args []string, env Env) error

func noCtx(f func([]string, Env) error) handler {
	return func(_ context.Context, args []string, env Env) error { return f(args, env) }
}

var commands = map[string]handler{
	"new":         noCtx(cmdNew),
	"notebook":    noCtx(cmdNotebook),
	"prompt":      noCtx(cmdPrompt),
	"usage":       noCtx(cmdUsage),
	"search":      noCtx(cmdSearch),
	"diagnostics": noCtx(cmdDiagnostics),
	"context":     noCtx(cmdContext),
	"run":         cmdRun,
	"outputs":     noCtx(cmdOutputs),
	"models":      cmdModels,
	"config":      noCtx(cmdConfig),
	"audit":       noCtx(cmdAudit),
	"serve":       cmdServe,
	"secret":      cmdSecret,
	"schedule":    noCtx(cmdSchedule),
	"template":    noCtx(cmdTemplate),
	"memory":      noCtx(cmdMemory),
	"chat":        cmdChat,
	"notify":      cmdNotify,
	"output":      cmdOutput,
	"mcp":         cmdMCP,
	"research":    cmdResearch,
	"worktypes":   func(_ context.Context, _ []string, env Env) error { return cmdWorkTypes(env) },
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
	}
	h, ok := commands[cmd]
	if !ok {
		return usagef("unknown command %q", cmd)
	}
	return h(ctx, rest, env)
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
