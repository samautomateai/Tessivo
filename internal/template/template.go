// Package template manages notebook and prompt templates: reusable setups
// (work type, prompts, context slots, memory seeds, settings, an optional
// schedule) that carry no data and no secrets. Built-in templates ship with
// Tessivo; user templates live as 0600 JSON files in the config directory.
package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/cron"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// Sources.
const (
	SourceBuiltin = "builtin"
	SourceUser    = "user"
)

// Limits keep templates small and safe to share.
const (
	MaxPrompts       = 20
	MaxSlots         = 10
	MaxMemory        = 50
	MaxTextBytes     = 16 << 10
	MaxFileBytes     = 256 << 10
	MaxUserTemplates = 500
)

// ErrNotFound means no template has the ID.
var ErrNotFound = errors.New("template not found")

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Prompt is a named prompt with optional settings.
type Prompt struct {
	Name     string             `json:"name"`
	Text     string             `json:"text"`
	Settings config.RunSettings `json:"settings"`
}

// Slot describes context the user adds when using the template.
type Slot struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"` // code base, diff, documents, spreadsheet, mcp, any
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Schedule is an optional schedule created with the notebook.
type Schedule struct {
	Cron       string `json:"cron"`
	PromptName string `json:"promptName,omitempty"`
	Missed     string `json:"missed"`
}

// Template is a reusable notebook setup.
type Template struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	WorkType    string             `json:"workType"`
	Version     int                `json:"version"`
	Parent      string             `json:"parent,omitempty"` // "id@version" this was extended from
	Prompts     []Prompt           `json:"prompts"`
	Slots       []Slot             `json:"slots"`
	Memory      []string           `json:"memory,omitempty"`
	Settings    config.RunSettings `json:"settings"`
	Schedule    *Schedule          `json:"schedule,omitempty"`
	Source      string             `json:"source"`
	UpdatedAt   string             `json:"updatedAt,omitempty"`
}

// Validate checks structure, limits and that no secrets slipped in.
func (t Template) Validate() error {
	var errs []error
	if !idPattern.MatchString(t.ID) {
		errs = append(errs, fmt.Errorf("id %q: use lowercase letters, digits and '-'", t.ID))
	}
	if strings.TrimSpace(t.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	if _, err := worktype.Get(t.WorkType); err != nil {
		errs = append(errs, err)
	}
	if len(t.Prompts) == 0 || len(t.Prompts) > MaxPrompts || len(t.Slots) > MaxSlots || len(t.Memory) > MaxMemory {
		errs = append(errs, fmt.Errorf("want 1-%d prompts, at most %d slots and %d memory items",
			MaxPrompts, MaxSlots, MaxMemory))
	}
	errs = append(errs, t.validateParts()...)
	return errors.Join(errs...)
}

func (t Template) validateParts() []error {
	var errs []error
	names := map[string]bool{}
	for _, p := range t.Prompts {
		if err := (notebook.Prompt{Name: p.Name, Text: p.Text, Settings: p.Settings}).Check(); err != nil {
			errs = append(errs, err)
		}
		names[p.Name] = true
		errs = append(errs, checkText("prompt "+p.Name, p.Text)...)
	}
	for _, m := range t.Memory {
		errs = append(errs, checkText("memory", m)...)
	}
	if err := t.Settings.Validate(); err != nil {
		errs = append(errs, err)
	}
	if s := t.Schedule; s != nil {
		if _, err := cron.Parse(s.Cron, time.Local); err != nil {
			errs = append(errs, err)
		}
		if s.Missed != notebook.MissedSkip && s.Missed != notebook.MissedRunOnce {
			errs = append(errs, fmt.Errorf("schedule missed policy %q: want skip or run_once", s.Missed))
		}
		if s.PromptName != "" && !names[s.PromptName] {
			errs = append(errs, fmt.Errorf("schedule prompt %q is not in the template", s.PromptName))
		}
	}
	return errs
}

func checkText(what, s string) []error {
	var errs []error
	if len(s) > MaxTextBytes {
		errs = append(errs, fmt.Errorf("%s is longer than %d bytes", what, MaxTextBytes))
	}
	if secrets.RedactPatterns(s) != s {
		errs = append(errs, fmt.Errorf("%s looks like it contains a secret; templates must not hold credentials", what))
	}
	return errs
}

// Library combines built-in and user templates.
type Library struct {
	Dir string // user templates folder
}

// List returns all templates sorted by name, built-ins first on ties.
func (l Library) List() ([]Template, error) {
	out := Builtins()
	user, err := l.user()
	if err != nil {
		return nil, err
	}
	out = append(out, user...)
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out, nil
}

func (l Library) user() ([]Template, error) {
	entries, err := os.ReadDir(l.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	var out []Template
	for i, e := range entries {
		if i >= MaxUserTemplates {
			break
		}
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() || !idPattern.MatchString(id) {
			continue
		}
		t, err := l.load(id)
		if err != nil {
			continue // a broken file must not hide the rest
		}
		out = append(out, t)
	}
	return out, nil
}

func (l Library) load(id string) (Template, error) {
	f, err := os.Open(filepath.Join(l.Dir, id+".json")) //nolint:gosec // G304: id matches idPattern
	if err != nil {
		return Template{}, fmt.Errorf("open template %s: %w", id, err)
	}
	defer func() { _ = f.Close() }()
	var t Template
	dec := json.NewDecoder(&limitedReader{r: f, n: MaxFileBytes})
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return Template{}, fmt.Errorf("parse template %s: %w", id, err)
	}
	t.Source = SourceUser
	if err := t.Validate(); err != nil {
		return Template{}, fmt.Errorf("template %s: %w", id, err)
	}
	return t, nil
}

// Get returns a template by ID.
func (l Library) Get(id string) (Template, error) {
	if t, ok := builtinByID(id); ok {
		return t, nil
	}
	if !idPattern.MatchString(id) {
		return Template{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	t, err := l.load(id)
	if errors.Is(err, os.ErrNotExist) {
		return Template{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return t, err
}

// Save writes a user template, bumping its version if it already exists.
func (l Library) Save(t Template) (Template, error) {
	if _, ok := builtinByID(t.ID); ok {
		return Template{}, fmt.Errorf("%q is a built-in template; extend it under a new id", t.ID)
	}
	t.Source, t.UpdatedAt = SourceUser, time.Now().UTC().Format(time.RFC3339)
	if prev, err := l.load(t.ID); err == nil {
		t.Version = prev.Version + 1
	} else if t.Version < 1 {
		t.Version = 1
	}
	if err := t.Validate(); err != nil {
		return Template{}, err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return Template{}, fmt.Errorf("encode template: %w", err)
	}
	if err := os.MkdirAll(l.Dir, 0o700); err != nil {
		return Template{}, fmt.Errorf("templates dir: %w", err)
	}
	path := filepath.Join(l.Dir, t.ID+".json")
	if err := os.WriteFile(path+".tmp", b, 0o600); err != nil {
		return Template{}, fmt.Errorf("write template: %w", err)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return Template{}, fmt.Errorf("replace template: %w", err)
	}
	return t, nil
}

// Delete removes a user template.
func (l Library) Delete(id string) error {
	if _, ok := builtinByID(id); ok {
		return fmt.Errorf("%q is built in and cannot be deleted", id)
	}
	if !idPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	if err := os.Remove(filepath.Join(l.Dir, id+".json")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, id)
		}
		return fmt.Errorf("delete template: %w", err)
	}
	return nil
}

type limitedReader struct {
	r interface{ Read([]byte) (int, error) }
	n int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errors.New("template file too large")
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}
