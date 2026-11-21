package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

type runOpts struct {
	workType, prompt, promptName, model, thinking, out, mode string
	vars, tools                                              multiFlag
	asJSON, noSave, quiet                                    bool
}

func cmdRun(ctx context.Context, args []string, env Env) error {
	if _, sub := runSubcommands[firstArg(args)]; sub {
		return cmdRunAgent(ctx, args[0], args[1:], env)
	}
	o, pos, err := parseRunFlags(args, env)
	if err != nil {
		return err
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	req, err := runRequest(o, pos, env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	a := &agentCLI{svc: svc, env: env, quiet: o.quiet, opts: o}
	if req.Notebook != nil {
		a.nbDir = req.Notebook.Dir
		svc.NotebookDirs = []string{req.Notebook.Dir}
	}
	out, err := svc.Run(ctx, req, a.onEvent)
	if err != nil {
		return cliErr(err)
	}
	return a.finish(ctx, out)
}

func runRequest(o runOpts, pos []string, env Env) (service.RunRequest, error) {
	req := service.RunRequest{
		WorkType: o.workType, Prompt: o.prompt, PromptName: o.promptName, Save: !o.noSave,
		Overrides: config.RunSettings{Model: o.model, ThinkingEffort: o.thinking, Mode: o.mode},
	}
	var err error
	if len(pos) == 1 && pos[0] != "-" {
		if req.Notebook, err = notebook.Open(pos[0]); err != nil {
			return req, err
		}
	}
	if req.Vars, err = parseVars(o.vars); err != nil {
		return req, err
	}
	if req.ToolRules, err = parseToolRules(o.tools); err != nil {
		return req, err
	}
	req.Extra, err = stdinDocs(env)
	return req, err
}

func parseRunFlags(args []string, env Env) (runOpts, []string, error) {
	o := runOpts{}
	fs := newFlags("run", env)
	fs.StringVar(&o.mode, "mode", "", "pipeline or agent (agent: tool-using; tools may ask for approval)")
	fs.Var(&o.tools, "tool", "agent tool rule name=allow|ask|block for this run (repeatable)")
	fs.StringVar(&o.workType, "type", "", "work type when running without a notebook")
	fs.StringVar(&o.prompt, "prompt", "", "prompt text (default: the work type's starter prompt)")
	fs.StringVar(&o.promptName, "prompt-name", "", "use a saved notebook prompt")
	fs.Var(&o.vars, "var", "prompt variable name=value (repeatable)")
	fs.StringVar(&o.model, "model", "", "model alias or provider/model")
	fs.StringVar(&o.thinking, "thinking", "", "thinking effort: off, low, medium, high")
	fs.StringVar(&o.out, "out", "", "write the output to this file instead of stdout")
	fs.BoolVar(&o.asJSON, "json", false, "print the structured JSON instead of Markdown")
	fs.BoolVar(&o.noSave, "no-save", false, "do not save the run and output in the notebook")
	fs.BoolVar(&o.quiet, "quiet", false, "do not stream progress to stderr or ask for approvals")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(env.Stderr, "Usage: tess run [<notebook>] [flags]\n"+
			"       tess run pending [--notebook DIR]\n"+
			"       tess run approve <run> <approval> [--remember run|notebook] [--notebook DIR]\n"+
			"       tess run deny|resume|cancel ... [--notebook DIR]\nFlags:")
		fs.PrintDefaults()
	}
	pos, err := parse(fs, args)
	if err != nil {
		return o, nil, err
	}
	if len(pos) > 1 {
		return o, nil, usagef("run: at most one notebook")
	}
	return o, pos, nil
}

func newService(f config.File, env Env) (*service.Service, error) {
	userAudit, err := auditPath(nil, env)
	if err != nil {
		return nil, err
	}
	ws, err := f.Workspace()
	if err != nil {
		return nil, err
	}
	lib, err := library(env)
	if err != nil {
		return nil, err
	}
	return &service.Service{
		Config: f, Secrets: env.Secrets, NewModel: service.ModelFactory(env.NewModel),
		Workspace: ws, UserAuditPath: userAudit, Actor: "cli", Templates: lib,
	}, nil
}

func stdinDocs(env Env) ([]extract.Document, error) {
	if env.StdinIsTTY || env.Stdin == nil {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(env.Stdin, extract.MaxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(b) > extract.MaxFileBytes {
		return nil, fmt.Errorf("stdin is larger than %d bytes", extract.MaxFileBytes)
	}
	if len(b) == 0 {
		return nil, nil
	}
	d, _ := extract.FromBytes("stdin", "text", b)
	return []extract.Document{d}, nil
}

func writeResult(out service.RunOutcome, o runOpts, env Env) error {
	text := out.Markdown
	if o.asJSON {
		text = string(out.Result.Data) + "\n"
	}
	if o.out != "" {
		if err := os.WriteFile(o.out, []byte(text), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", o.out, err)
		}
		return nil
	}
	_, err := io.WriteString(env.Stdout, text)
	return wrapWrite(err)
}
