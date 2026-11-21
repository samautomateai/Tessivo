package cli

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/service"
)

// pausedError ends a command whose agent run waits for approval (exit 4).
type pausedError struct{ msg string }

func (e pausedError) Error() string { return e.msg }

// runSubcommands are the words that mean a subcommand, not a notebook, with their argument count; a notebook
// with one of these names is reached as ./name.
var runSubcommands = map[string]int{"pending": 0, subApprove: 2, subDeny: 2, "resume": 1, "cancel": 1}

func cmdRunAgent(ctx context.Context, sub string, args []string, env Env) error {
	fs := newFlags("run "+sub, env)
	nbDir := fs.String("notebook", "", "notebook folder outside the workspace that holds the run")
	remember := fs.String("remember", "", "approve only: also allow later calls in this run or notebook")
	quiet := fs.Bool("quiet", false, "do not stream progress or ask in the terminal")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	need := runSubcommands[sub]
	if len(pos) != need {
		return usagef("run %s: want %d argument(s)", sub, need)
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	if *nbDir != "" {
		abs, err := filepath.Abs(*nbDir)
		if err != nil {
			return fmt.Errorf("notebook %s: %w", *nbDir, err)
		}
		svc.NotebookDirs = []string{abs}
	}
	a := &agentCLI{svc: svc, env: env, quiet: *quiet, nbDir: *nbDir}
	switch sub {
	case "pending":
		return a.pending()
	case "cancel":
		return cliErr(svc.Cancel(pos[0]))
	case "resume":
		return a.resume(ctx, pos[0])
	case subDeny:
		*remember = ""
	}
	answer := map[string]string{subApprove: config.RuleAllow, subDeny: answerDeny}[sub]
	if err := svc.Answer(pos[0], pos[1], answer, *remember); err != nil {
		return cliErr(err)
	}
	return a.resume(ctx, pos[0])
}

// agentCLI drives agent runs from the terminal.
type agentCLI struct {
	svc   *service.Service
	env   Env
	quiet bool
	nbDir string
	in    *bufio.Reader
	opts  runOpts
}

func (a *agentCLI) onEvent(e executor.Event) {
	if !a.quiet && (e.Kind == executor.EventStatus || e.Kind == executor.EventToolCall) {
		_, _ = fmt.Fprintf(a.env.Stderr, "· %s\n", e.Text)
	}
}

func (a *agentCLI) resume(ctx context.Context, runID string) error {
	out, err := a.svc.Resume(ctx, runID, a.onEvent)
	if err != nil {
		return cliErr(err)
	}
	return a.finish(ctx, out)
}

// finish prints a finished run, or handles its pause: asks in a terminal, or
// explains how to answer and exits with ExitPaused.
func (a *agentCLI) finish(ctx context.Context, out service.RunOutcome) error {
	for len(out.Paused) > 0 {
		p := out.Paused[0]
		answer, remember, ok := a.ask(p)
		if !ok {
			return pausedError{a.pauseMessage(p)}
		}
		if err := a.svc.Answer(p.RunID, p.ApprovalID, answer, remember); err != nil {
			return cliErr(err)
		}
		var err error
		if out, err = a.svc.Resume(ctx, p.RunID, a.onEvent); err != nil {
			return cliErr(err)
		}
	}
	return writeResult(out, a.opts, a.env)
}

// ask prompts in an interactive terminal. ok is false for "later", end of
// input, or when not interactive.
func (a *agentCLI) ask(p service.PendingApproval) (answer, remember string, ok bool) {
	if !a.env.StdinIsTTY || a.quiet || a.env.Stdin == nil {
		return "", "", false
	}
	if a.in == nil {
		a.in = bufio.NewReader(a.env.Stdin)
	}
	_, _ = fmt.Fprintf(a.env.Stderr, "allow %s %s? [y] once  [r] this run  [n] this notebook  [d] deny  [l] later: ",
		p.Tool, p.Args)
	line, _ := a.in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y":
		return config.RuleAllow, "", true
	case "r":
		return config.RuleAllow, service.RememberRun, true
	case "n":
		return config.RuleAllow, service.RememberNotebook, true
	case "d":
		return answerDeny, "", true
	}
	return "", "", false
}

func (a *agentCLI) pauseMessage(p service.PendingApproval) string {
	where := ""
	if a.nbDir != "" {
		where = " --notebook " + a.nbDir
	}
	return fmt.Sprintf("paused: run %s needs approval %s (%s %s)\n  allow: tess run approve %s %s%s\n"+
		"  deny:  tess run deny %s %s%s", p.RunID, p.ApprovalID, p.Tool, p.Args,
		p.RunID, p.ApprovalID, where, p.RunID, p.ApprovalID, where)
}

func (a *agentCLI) pending() error {
	list, err := a.svc.Pending()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		_, err := fmt.Fprintln(a.env.Stdout, "no approvals waiting")
		return wrapWrite(err)
	}
	tw := tabwriter.NewWriter(a.env.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "RUN\tAPPROVAL\tNOTEBOOK\tTOOL\tREQUESTED\tEXPIRES")
	for _, p := range list {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", p.RunID, p.ApprovalID, p.NotebookName, p.Tool,
			p.RequestedAt, p.ExpiresAt)
	}
	return wrapWrite(tw.Flush())
}

// parseToolRules turns --tool name=rule flags into rules.
func parseToolRules(vs []string) (map[string]string, error) {
	if len(vs) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, v := range vs {
		name, rule, ok := strings.Cut(v, "=")
		if !ok || name == "" || !config.ValidRule(rule) {
			return nil, usagef("--tool %q: want name=allow, name=ask or name=block", v)
		}
		out[name] = rule
	}
	return out, nil
}

const (
	subApprove = "approve"
	subDeny    = "deny"
	answerDeny = "deny" // the answer word the service expects
)

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
