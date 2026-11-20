package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

type runOpts struct {
	workType, prompt, promptName, model, thinking, out string
	vars                                               multiFlag
	asJSON, noSave, quiet                              bool
}

func cmdRun(ctx context.Context, args []string, env Env) error {
	o := runOpts{}
	fs := newFlags("run", env)
	fs.StringVar(&o.workType, "type", "", "work type when running without a notebook")
	fs.StringVar(&o.prompt, "prompt", "", "prompt text (default: the work type's starter prompt)")
	fs.StringVar(&o.promptName, "prompt-name", "", "use a saved notebook prompt")
	fs.Var(&o.vars, "var", "prompt variable name=value (repeatable)")
	fs.StringVar(&o.model, "model", "", "model alias or provider/model")
	fs.StringVar(&o.thinking, "thinking", "", "thinking effort: off, low, medium, high")
	fs.StringVar(&o.out, "out", "", "write the output to this file instead of stdout")
	fs.BoolVar(&o.asJSON, "json", false, "print the structured JSON instead of Markdown")
	fs.BoolVar(&o.noSave, "no-save", false, "do not save the run and output in the notebook")
	fs.BoolVar(&o.quiet, "quiet", false, "do not stream progress to stderr")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) > 1 {
		return usagef("run: at most one notebook")
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	req := service.RunRequest{
		WorkType: o.workType, Prompt: o.prompt, PromptName: o.promptName, Save: !o.noSave,
		Overrides: config.RunSettings{Model: o.model, ThinkingEffort: o.thinking},
	}
	if len(pos) == 1 && pos[0] != "-" {
		if req.Notebook, err = notebook.Open(pos[0]); err != nil {
			return err
		}
	}
	if req.Vars, err = parseVars(o.vars); err != nil {
		return err
	}
	if req.Extra, err = stdinDocs(env); err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	onEvent := func(e executor.Event) {
		if !o.quiet && e.Kind == executor.EventStatus {
			_, _ = fmt.Fprintf(env.Stderr, "· %s\n", e.Text)
		}
	}
	out, err := svc.Run(ctx, req, onEvent)
	if err != nil {
		if service.IsInvalid(err) {
			return usageError{err.Error()}
		}
		return err
	}
	return writeResult(out, o, env)
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
	return &service.Service{
		Config: f, Secrets: env.Secrets, NewModel: service.ModelFactory(env.NewModel),
		Workspace: ws, UserAuditPath: userAudit, Actor: "cli",
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
