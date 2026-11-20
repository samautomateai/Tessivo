// Package service is Tessivo's application layer: the operations the CLI and
// the local API server both expose (notebooks, context, runs, outputs,
// settings). Keeping them here means the two front ends cannot drift.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// InvalidError marks a bad request (CLI exit 2, HTTP 400).
type InvalidError struct{ Msg string }

func (e *InvalidError) Error() string { return e.Msg }

func invalidf(format string, a ...any) error { return &InvalidError{fmt.Sprintf(format, a...)} }

// IsInvalid reports whether err is an InvalidError.
func IsInvalid(err error) bool {
	var ie *InvalidError
	return errors.As(err, &ie)
}

// ErrNotFound marks a missing notebook or output (HTTP 404).
var ErrNotFound = errors.New("not found")

// ModelFactory builds the model for an alias.
type ModelFactory func(f config.File, alias config.Alias, sec secrets.Resolver) (executor.Model, error)

// Service holds configuration and collaborators. It is safe for concurrent
// use as long as callers do not mutate Config.
type Service struct {
	Config        config.File
	Secrets       secrets.Resolver
	NewModel      ModelFactory
	Workspace     string // folder holding notebooks addressed by ID
	UserAuditPath string // audit log for runs without a notebook
	Actor         string // cli, desktop, scheduler
}

// DefaultModelFactory builds an llm.Router.
func DefaultModelFactory(f config.File, a config.Alias, s secrets.Resolver) (executor.Model, error) {
	return llm.NewRouter(f, a, s)
}

// RunRequest describes one run.
type RunRequest struct {
	Notebook   *notebook.Notebook // nil: notebook-less run (needs WorkType)
	WorkType   string
	Prompt     string
	PromptName string
	Vars       map[string]string
	Overrides  config.RunSettings
	Extra      []extract.Document // e.g. piped stdin
	Save       bool
}

// RunOutcome is a finished run.
type RunOutcome struct {
	RunID    string
	OutputID string
	Result   executor.Result
	Markdown string
	Spec     executor.Spec
}

// Run executes a work type against the configured model, audits it and, if
// requested, saves the run record and output in the notebook.
func (s *Service) Run(ctx context.Context, req RunRequest, onEvent func(executor.Event)) (RunOutcome, error) {
	spec, err := s.buildSpec(req)
	if err != nil {
		return RunOutcome{}, err
	}
	alias, err := s.Config.ResolveModel(spec.Settings.Model)
	if err != nil {
		return RunOutcome{}, &InvalidError{err.Error()}
	}
	factory := s.NewModel
	if factory == nil {
		factory = DefaultModelFactory
	}
	model, err := factory(s.Config, alias, s.Secrets)
	if err != nil {
		return RunOutcome{}, err
	}
	rec, err := s.startAudit(req.Notebook, spec)
	if err != nil {
		return RunOutcome{}, fmt.Errorf("audit: %w", err)
	}
	started := time.Now()
	res, runErr := (&executor.Pipeline{Model: model}).Run(ctx, spec, onEvent)
	out := RunOutcome{RunID: rec.runID, Result: res, Spec: spec}
	if req.Notebook != nil && req.Save {
		id, err := saveRun(req.Notebook, rec.runID, spec, res, runErr, started)
		if err != nil {
			return out, errors.Join(runErr, err, rec.finish(res, runErr, "", time.Since(started)))
		}
		out.OutputID = id
	}
	if err := rec.finish(res, runErr, out.OutputID, time.Since(started)); err != nil {
		return out, errors.Join(runErr, fmt.Errorf("audit: %w", err))
	}
	if runErr != nil {
		return out, runErr
	}
	out.Markdown, err = render.Markdown(res.Data, RequiredOrder(spec.WorkType))
	return out, err
}

func (s *Service) buildSpec(req RunRequest) (executor.Spec, error) {
	wtID, prompt := req.WorkType, req.Prompt
	layers := []config.Layer{{Name: "app", Settings: s.Config.Defaults}}
	if nb := req.Notebook; nb != nil {
		wtID = nb.Manifest.WorkType
		layers = append(layers, config.Layer{Name: "notebook", Settings: nb.Manifest.Settings})
		if req.PromptName != "" {
			p, err := nb.LoadPrompt(req.PromptName)
			if err != nil {
				return executor.Spec{}, err
			}
			prompt = p.Text
			layers = append(layers, config.Layer{Name: "prompt", Settings: p.Settings})
		}
	}
	if wtID == "" {
		return executor.Spec{}, invalidf("give a notebook or a work type")
	}
	wt, err := worktype.Get(wtID)
	if err != nil {
		return executor.Spec{}, &InvalidError{err.Error()}
	}
	if prompt == "" {
		prompt = wt.StarterPrompt
	}
	if err := req.Overrides.Validate(); err != nil {
		return executor.Spec{}, &InvalidError{err.Error()}
	}
	layers = append(layers, config.Layer{Name: "run", Settings: req.Overrides})
	docs, err := notebookDocs(req.Notebook)
	if err != nil {
		return executor.Spec{}, err
	}
	spec := executor.Spec{
		WorkType: wt, Prompt: prompt, Vars: req.Vars, Documents: append(docs, req.Extra...),
		Settings: config.Resolve(layers...),
	}
	if spec.Settings.Model == "" {
		return executor.Spec{}, invalidf("no model set; pass a model or set defaults.model in the config")
	}
	return spec, nil
}

func notebookDocs(nb *notebook.Notebook) ([]extract.Document, error) {
	if nb == nil {
		return nil, nil
	}
	var docs []extract.Document
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
	return docs, nil
}

// RequiredOrder is the work type's schema property order, for rendering.
func RequiredOrder(wt worktype.WorkType) []string {
	s, err := wt.ParsedSchema()
	if err != nil {
		return nil
	}
	return s.Required
}
