// Package service is Tessivo's application layer: the operations the CLI and
// the local API server both expose (notebooks, context, runs, outputs,
// settings). Keeping them here means the two front ends cannot drift.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/render"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/template"
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
	Templates     template.Library
	ChatDir       string // chat sessions and memory; default: "chat" next to Workspace
	ConfigPath    string // the config file MCP server changes are written to; "" keeps them in memory

	// NotebookDirs are notebook folders outside Workspace to search for agent
	// runs (the CLI passes --notebook).
	NotebookDirs []string
	// OnRunEvent receives agent run events (approval needed, finished, ...).
	OnRunEvent func(RunEvent)
	// LogPath is the core log (tess serve), included in diagnostics; "" when there is none.
	LogPath string
	// SecretStore stores API keys set from the desktop (the OS keychain); nil: none.
	SecretStore SecretStore
	// Now is the clock; nil means time.Now.
	Now func() time.Time

	cfgMu     sync.RWMutex      // guards Config, which the API can change; read it through cfg()
	packMu    sync.Mutex        // guards packErrs and pack refreshes
	packErrs  map[string]string // the last refresh error of each pack
	sumOnce   sync.Once         // starts the research summary runner on first use
	sum       *summaryRunner
	checkOnce sync.Once // creates checks on first use
	checks    *checkTable
	usageMu   sync.Mutex // serializes usage.json updates in this process
	jobsOnce  sync.Once
	jobs      *jobTable
	deliverMu sync.Mutex // serializes delivery records (deliver.go)
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
	Actor      string            // overrides Service.Actor in the audit log (e.g. scheduler)
	ToolRules  map[string]string // agent mode: rules for this run only (most specific layer)
	ScheduleID string            // the schedule that started the run, if any
	Detached   bool              // agent mode: the run outlives ctx; stop it with Cancel
	MaxTokens  int               // this run's token limit (a schedule's maxTokens); 0: limits.max_tokens_per_run
}

// RunOutcome is a finished run.
type RunOutcome struct {
	RunID    string
	OutputID string
	Result   executor.Result
	Markdown string
	Spec     executor.Spec
	Paused   []PendingApproval // agent mode: set when the run stopped to wait for approval
}

// Run executes a work type against the configured model, audits it and, if
// requested, saves the run record and output in the notebook.
func (s *Service) Run(ctx context.Context, req RunRequest, onEvent func(executor.Event)) (RunOutcome, error) {
	spec, err := s.buildSpec(ctx, req)
	if err != nil {
		return RunOutcome{}, err
	}
	model, err := s.modelFor(spec)
	if err != nil {
		return RunOutcome{}, err
	}
	model = s.capRun(model, req)
	if spec.Settings.Mode == config.ModeAgent {
		return s.runAgent(ctx, req, spec, model, onEvent)
	}
	return s.runPipeline(ctx, req, spec, model, onEvent)
}

// runPipeline runs spec through the pipeline, audits it and, if requested,
// saves the run record and output.
func (s *Service) runPipeline(ctx context.Context, req RunRequest, spec executor.Spec, model executor.Model,
	onEvent func(executor.Event),
) (RunOutcome, error) {
	rec, err := s.startAudit(req.Notebook, spec, req.Actor)
	if err != nil {
		return RunOutcome{}, fmt.Errorf("audit: %w", err)
	}
	started := time.Now()
	res, runErr := (&executor.Pipeline{Model: model}).Run(ctx, spec, onEvent)
	out := RunOutcome{RunID: rec.runID, Result: res, Spec: spec}
	if req.Notebook != nil && req.Save {
		id, err := saveRun(req.Notebook, rec.runID, spec, res, runErr, started, req.decorate)
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

func (s *Service) modelFor(spec executor.Spec) (executor.Model, error) {
	alias, err := s.cfg().ResolveModel(spec.Settings.Model)
	if err != nil {
		return nil, &InvalidError{err.Error()}
	}
	return s.aliasModel(alias)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) buildSpec(ctx context.Context, req RunRequest) (executor.Spec, error) {
	wtID, prompt := req.WorkType, req.Prompt
	layers := []config.Layer{{Name: layerApp, Settings: s.cfg().Defaults}}
	if nb := req.Notebook; nb != nil {
		wtID = nb.Manifest.WorkType
		layers = append(layers, config.Layer{Name: layerNotebook, Settings: nb.Manifest.Settings})
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
	docs, err := s.notebookDocs(ctx, req.Notebook)
	if err != nil {
		return executor.Spec{}, err
	}
	spec := executor.Spec{
		WorkType: wt, Prompt: prompt, Vars: withPrevious(req.Notebook, prompt, req.Vars),
		Documents: append(docs, req.Extra...),
		Settings:  config.Resolve(layers...),
	}
	if req.Notebook != nil {
		spec.Memory = req.Notebook.Manifest.Memory
	}
	if spec.Settings.Model == "" {
		return executor.Spec{}, invalidf("no model set; pass a model or set defaults.model in the config")
	}
	return spec, nil
}

// RequiredOrder is the work type's schema property order, for rendering.
func RequiredOrder(wt worktype.WorkType) []string {
	s, err := wt.ParsedSchema()
	if err != nil {
		return nil
	}
	return s.Required
}

// decorate records where the run's prompt came from, so later prompt changes
// reach the right saved prompt or schedule.
func (req RunRequest) decorate(r *notebook.Run) {
	r.PromptName = req.PromptName
	if req.Notebook != nil && req.PromptName != "" {
		if p, err := req.Notebook.LoadPrompt(req.PromptName); err == nil {
			r.PromptVersion = max(p.Version, 1)
		}
	}
	if r.ScheduleID == "" {
		r.ScheduleID = req.ScheduleID
	}
}

// cfg is the current config. Writers replace whole blocks (new slices and
// maps), never edit them in place, so a copy is safe to read without the lock.
func (s *Service) cfg() config.File {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.Config
}
