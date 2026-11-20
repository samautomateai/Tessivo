package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
	"github.com/samautomateai/tessivo/internal/worktype"
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
	var nb *notebook.Notebook
	if len(pos) == 1 && pos[0] != "-" {
		if nb, err = notebook.Open(pos[0]); err != nil {
			return err
		}
	}
	spec, layers, err := buildSpec(nb, o, env)
	if err != nil {
		return err
	}
	return execute(ctx, f, nb, spec, layers, o, env)
}

func buildSpec(nb *notebook.Notebook, o runOpts, env Env) (executor.Spec, []config.Layer, error) {
	wtID := o.workType
	var layers []config.Layer
	prompt := o.prompt
	if nb != nil {
		wtID = nb.Manifest.WorkType
		layers = append(layers, config.Layer{Name: "notebook", Settings: nb.Manifest.Settings})
		if o.promptName != "" {
			p, err := nb.LoadPrompt(o.promptName)
			if err != nil {
				return executor.Spec{}, nil, err
			}
			prompt = p.Text
			layers = append(layers, config.Layer{Name: "prompt", Settings: p.Settings})
		}
	}
	if wtID == "" {
		return executor.Spec{}, nil, usagef("run: give a notebook or --type")
	}
	wt, err := worktype.Get(wtID)
	if err != nil {
		return executor.Spec{}, nil, err
	}
	if prompt == "" {
		prompt = wt.StarterPrompt
	}
	vars, err := parseVars(o.vars)
	if err != nil {
		return executor.Spec{}, nil, err
	}
	run := config.RunSettings{Model: o.model, ThinkingEffort: o.thinking}
	if err := run.Validate(); err != nil {
		return executor.Spec{}, nil, usageError{err.Error()}
	}
	layers = append(layers, config.Layer{Name: "run", Settings: run})
	docs, err := gatherDocs(nb, env)
	if err != nil {
		return executor.Spec{}, nil, err
	}
	return executor.Spec{WorkType: wt, Prompt: prompt, Vars: vars, Documents: docs}, layers, nil
}

func gatherDocs(nb *notebook.Notebook, env Env) ([]extract.Document, error) {
	var docs []extract.Document
	if nb != nil {
		for _, c := range nb.Manifest.Context {
			if !c.Enabled {
				continue
			}
			d, err := extract.Path(nb.ContextPath(c))
			if err != nil {
				return nil, err
			}
			docs = append(docs, d...)
		}
	}
	if !env.StdinIsTTY && env.Stdin != nil {
		b, err := io.ReadAll(io.LimitReader(env.Stdin, extract.MaxFileBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		if len(b) > extract.MaxFileBytes {
			return nil, fmt.Errorf("stdin is larger than %d bytes", extract.MaxFileBytes)
		}
		if len(b) > 0 {
			d, _ := extract.FromBytes("stdin", "text", b)
			docs = append(docs, d)
		}
	}
	return docs, nil
}

func execute(ctx context.Context, f config.File, nb *notebook.Notebook, spec executor.Spec, layers []config.Layer,
	o runOpts, env Env,
) error {
	all := append([]config.Layer{{Name: "app", Settings: f.Defaults}}, layers...)
	spec.Settings = config.Resolve(all...)
	if spec.Settings.Model == "" {
		return usagef("run: no model set; pass --model or set defaults.model in the config")
	}
	alias, err := f.ResolveModel(spec.Settings.Model)
	if err != nil {
		return err
	}
	model, err := env.NewModel(f, alias, env.Secrets)
	if err != nil {
		return err
	}
	onEvent := func(e executor.Event) {
		if !o.quiet && e.Kind == executor.EventStatus {
			_, _ = fmt.Fprintf(env.Stderr, "· %s\n", e.Text)
		}
	}
	started := time.Now()
	res, runErr := (&executor.Pipeline{Model: model}).Run(ctx, spec, onEvent)
	if nb != nil && !o.noSave {
		if err := saveRun(nb, spec, res, runErr, started); err != nil {
			return errors.Join(runErr, err)
		}
	}
	if runErr != nil {
		return runErr
	}
	return writeResult(spec, res, o, env)
}

func writeResult(spec executor.Spec, res executor.Result, o runOpts, env Env) error {
	text := string(res.Data) + "\n"
	if !o.asJSON {
		md, err := render.Markdown(res.Data, requiredOrder(spec.WorkType))
		if err != nil {
			return err
		}
		text = md
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

func requiredOrder(wt worktype.WorkType) []string {
	s, err := wt.ParsedSchema()
	if err != nil {
		return nil
	}
	return s.Required
}

func saveRun(nb *notebook.Notebook, spec executor.Spec, res executor.Result, runErr error, started time.Time) error {
	run := notebook.Run{
		ID: notebook.NewID(), StartedAt: started.UTC().Format(time.RFC3339), WorkType: spec.WorkType.ID,
		DurationMS: time.Since(started).Milliseconds(), Prompt: spec.Prompt, Vars: spec.Vars, Settings: spec.Settings,
		ModelUsed: res.Model, Strategy: res.Strategy, TotalTokens: res.TotalTokens, Valid: res.Valid,
	}
	for _, d := range spec.Documents {
		run.Context = append(run.Context, d.Source)
	}
	for _, a := range res.Attempts {
		ar := notebook.AttemptRecord{Target: a.Target, Trigger: a.Trigger}
		if a.Err != nil {
			ar.Error = a.Err.Error()
		}
		run.Attempts = append(run.Attempts, ar)
	}
	if runErr != nil {
		run.Error = runErr.Error()
	}
	if res.Valid {
		out := notebook.Output{
			ID: notebook.NewID(), RunID: run.ID, WorkType: spec.WorkType.ID,
			Title: titleOf(res.Data), CreatedAt: run.StartedAt, Data: res.Data,
		}
		md, _ := render.Markdown(res.Data, requiredOrder(spec.WorkType))
		if err := nb.SaveOutput(out, md); err != nil {
			return err
		}
		run.OutputID = out.ID
	}
	return nb.SaveRun(run)
}

func titleOf(data json.RawMessage) string {
	var v struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal(data, &v)
	return v.Title
}
