package notebook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
)

// Prompt is a saved, reusable prompt with {{variables}}.
type Prompt struct {
	Name     string             `json:"name"`
	Text     string             `json:"text"`
	Settings config.RunSettings `json:"settings"`
}

// AttemptRecord is one model attempt within a run.
type AttemptRecord struct {
	Target  string `json:"target"`
	Trigger string `json:"trigger,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Run records exactly what a run used and produced.
type Run struct {
	ID          string            `json:"id"`
	StartedAt   string            `json:"startedAt"`
	DurationMS  int64             `json:"durationMs"`
	WorkType    string            `json:"workType"`
	Prompt      string            `json:"prompt"`
	PromptName  string            `json:"promptName,omitempty"` // the saved prompt the run used, if any
	Vars        map[string]string `json:"vars,omitempty"`
	Context     []string          `json:"context"`
	Settings    config.Effective  `json:"settings"`
	ModelUsed   string            `json:"modelUsed"`
	Attempts    []AttemptRecord   `json:"attempts"`
	Strategy    string            `json:"strategy"` // full or map-reduce
	TotalTokens int               `json:"totalTokens"`
	Valid       bool              `json:"valid"`
	Error       string            `json:"error,omitempty"`
	OutputID    string            `json:"outputId,omitempty"`
	Mode        string            `json:"mode,omitempty"`   // pipeline or agent
	Status      string            `json:"status,omitempty"` // one of the Status constants
	ScheduleID  string            `json:"scheduleId,omitempty"`
	Steps       int               `json:"steps,omitempty"`
	ToolCalls   []ToolCallRecord  `json:"toolCalls,omitempty"`
	Approvals   []ApprovalRecord  `json:"approvals,omitempty"`
}

// Run statuses.
const (
	StatusRunning     = "running"
	StatusWaiting     = "waiting_approval"
	StatusInterrupted = "interrupted"
	StatusFinished    = "finished"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)

// ToolCallRecord is one tool call of an agent run. Args and Result are
// redacted and cut to 8 KiB.
type ToolCallRecord struct {
	ID         string `json:"id"`
	Tool       string `json:"tool"`
	Args       string `json:"args,omitempty"`
	Result     string `json:"result,omitempty"`
	Policy     string `json:"policy,omitempty"`   // allow, ask or block
	Decision   string `json:"decision,omitempty"` // allow or deny when the user was asked
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
}

// ApprovalRecord is one question put to the user and its answer.
type ApprovalRecord struct {
	ID          string `json:"id"`
	Tool        string `json:"tool"`
	Args        string `json:"args,omitempty"`
	RequestedAt string `json:"requestedAt"`
	Answer      string `json:"answer,omitempty"` // allow or deny
	AnsweredAt  string `json:"answeredAt,omitempty"`
	Remember    string `json:"remember,omitempty"` // "", run or notebook
}

const maxRecordText = 8 << 10

// TruncateRecord cuts s to 8 KiB for a run record, marking the cut.
func TruncateRecord(s string) string {
	if len(s) <= maxRecordText {
		return s
	}
	return s[:maxRecordText] + "…[truncated]"
}

// Output is a saved, structured result filed by work type.
type Output struct {
	ID         string          `json:"id"`
	RunID      string          `json:"runId"`
	WorkType   string          `json:"workType"`
	Title      string          `json:"title"`
	CreatedAt  string          `json:"createdAt"`
	Data       json.RawMessage `json:"data"`
	ParentID   string          `json:"parentId,omitempty"`   // the output this version revises
	FromThread string          `json:"fromThread,omitempty"` // the thread message the version was made from
}

// GetOutput returns one output and its Markdown (empty if none was saved).
// A missing output wraps fs.ErrNotExist.
func (n *Notebook) GetOutput(id string) (Output, string, error) {
	if !ValidID(id) {
		return Output{}, "", fmt.Errorf("output id %q is not valid", id)
	}
	var o Output
	if err := readJSON(filepath.Join(n.Dir, OutputsDir, id+".json"), &o); err != nil {
		return Output{}, "", err
	}
	md, err := os.ReadFile(filepath.Join(n.Dir, OutputsDir, id+".md")) //nolint:gosec // G304: validated ID
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Output{}, "", fmt.Errorf("read output markdown: %w", err)
	}
	return o, string(md), nil
}

// Check validates the prompt name, text and settings.
func (p Prompt) Check() error {
	if !safeName.MatchString(p.Name) {
		return fmt.Errorf("prompt name %q: use letters, digits, '.', '_' or '-'", p.Name)
	}
	if strings.TrimSpace(p.Text) == "" || len(p.Text) > MaxPromptText {
		return fmt.Errorf("prompt %s: text must be 1 to %d bytes", p.Name, MaxPromptText)
	}
	if err := p.Settings.Validate(); err != nil {
		return fmt.Errorf("prompt %s settings: %w", p.Name, err)
	}
	return nil
}

// SavePrompt writes prompts/<name>.json.
func (n *Notebook) SavePrompt(p Prompt) error {
	if err := p.Check(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(n.Dir, PromptsDir, p.Name+".json"), p)
}

// LoadPrompt reads a saved prompt.
func (n *Notebook) LoadPrompt(name string) (Prompt, error) {
	if !safeName.MatchString(name) {
		return Prompt{}, fmt.Errorf("prompt name %q is not valid", name)
	}
	var p Prompt
	err := readJSON(filepath.Join(n.Dir, PromptsDir, name+".json"), &p)
	return p, err
}

// DeletePrompt removes a saved prompt.
func (n *Notebook) DeletePrompt(name string) error {
	if !safeName.MatchString(name) {
		return fmt.Errorf("prompt name %q is not valid", name)
	}
	if err := os.Remove(filepath.Join(n.Dir, PromptsDir, name+".json")); err != nil {
		return fmt.Errorf("delete prompt %s: %w", name, err)
	}
	return nil
}

// ListPrompts returns saved prompt names, sorted.
func (n *Notebook) ListPrompts() ([]string, error) {
	return listJSON(filepath.Join(n.Dir, PromptsDir))
}

// SaveRun writes runs/<id>.json.
func (n *Notebook) SaveRun(r Run) error {
	return writeJSON(filepath.Join(n.Dir, RunsDir, r.ID+".json"), r)
}

// SaveOutput writes outputs/<id>.json and, if markdown is non-empty, outputs/<id>.md.
func (n *Notebook) SaveOutput(o Output, markdown string) error {
	if err := writeJSON(filepath.Join(n.Dir, OutputsDir, o.ID+".json"), o); err != nil {
		return err
	}
	if markdown == "" {
		return nil
	}
	if err := os.WriteFile(filepath.Join(n.Dir, OutputsDir, o.ID+".md"), []byte(markdown), 0o600); err != nil {
		return fmt.Errorf("write output markdown: %w", err)
	}
	return nil
}

// ListOutputs returns saved outputs, newest first.
func (n *Notebook) ListOutputs() ([]Output, error) {
	ids, err := listJSON(filepath.Join(n.Dir, OutputsDir))
	if err != nil {
		return nil, err
	}
	out := make([]Output, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		var o Output
		if err := readJSON(filepath.Join(n.Dir, OutputsDir, ids[i]+".json"), &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func listJSON(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", filepath.Base(dir), err)
	}
	var names []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".json"); ok && !e.IsDir() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}
