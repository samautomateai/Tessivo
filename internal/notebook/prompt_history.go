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

// MaxPromptVersions is how many versions of a prompt are kept, the current one included.
const MaxPromptVersions = 50

// promptHistoryDir holds earlier versions: prompts/.history/<name>/<version>.json.
const promptHistoryDir = ".history"

// PromptVersion is one saved version of a prompt.
type PromptVersion struct {
	Version  int                `json:"version"`
	SavedAt  string             `json:"savedAt,omitempty"`
	Text     string             `json:"text"`
	Settings config.RunSettings `json:"settings"`
}

func (n *Notebook) historyDir(name string) string {
	return filepath.Join(n.Dir, PromptsDir, promptHistoryDir, name)
}

// keepVersion is called before a prompt is overwritten: when the text or
// settings change, the old prompt goes to its history and p gets the next
// version; an unchanged save keeps the version.
func (n *Notebook) keepVersion(p *Prompt) error {
	old, err := n.LoadPrompt(p.Name)
	if errors.Is(err, fs.ErrNotExist) {
		p.Version = 1
		return nil
	}
	if err != nil {
		return err
	}
	if old.Version == 0 {
		old.Version = 1 // saved before versions existed
	}
	if old.Text == p.Text && sameSettings(old.Settings, p.Settings) {
		p.Version, p.SavedAt = old.Version, old.SavedAt
		return nil
	}
	v := PromptVersion{Version: old.Version, SavedAt: old.SavedAt, Text: old.Text, Settings: old.Settings}
	if err := os.MkdirAll(n.historyDir(p.Name), 0o700); err != nil {
		return fmt.Errorf("prompt history: %w", err)
	}
	if err := writeJSON(filepath.Join(n.historyDir(p.Name), fmt.Sprintf("%06d.json", v.Version)), v); err != nil {
		return fmt.Errorf("prompt history: %w", err)
	}
	p.Version = old.Version + 1
	return n.pruneHistory(p.Name)
}

func sameSettings(a, b config.RunSettings) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// pruneHistory keeps the newest MaxPromptVersions-1 earlier versions.
func (n *Notebook) pruneHistory(name string) error {
	files, err := n.historyFiles(name)
	if err != nil {
		return err
	}
	for len(files) > MaxPromptVersions-1 {
		if err := os.Remove(filepath.Join(n.historyDir(name), files[0])); err != nil {
			return fmt.Errorf("prompt history: %w", err)
		}
		files = files[1:]
	}
	return nil
}

// historyFiles lists the version files, oldest first.
func (n *Notebook) historyFiles(name string) ([]string, error) {
	entries, err := os.ReadDir(n.historyDir(name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prompt history: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out) // zero-padded version numbers sort in order
	return out, nil
}

// PromptVersions returns the prompt's versions, newest (the current one) first.
func (n *Notebook) PromptVersions(name string) ([]PromptVersion, error) {
	cur, err := n.LoadPrompt(name)
	if errors.Is(err, fs.ErrNotExist) {
		return []PromptVersion{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []PromptVersion{{Version: max(cur.Version, 1), SavedAt: cur.SavedAt, Text: cur.Text, Settings: cur.Settings}}
	files, err := n.historyFiles(name)
	if err != nil {
		return nil, err
	}
	for i := len(files) - 1; i >= 0; i-- {
		var v PromptVersion
		if err := readJSON(filepath.Join(n.historyDir(name), files[i]), &v); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

// RestorePromptVersion saves an earlier version as the prompt's new current
// version; the one it replaces stays in the history.
func (n *Notebook) RestorePromptVersion(name string, version int) error {
	vs, err := n.PromptVersions(name)
	if err != nil {
		return err
	}
	for _, v := range vs {
		if v.Version == version {
			return n.SavePrompt(Prompt{Name: name, Text: v.Text, Settings: v.Settings})
		}
	}
	return fmt.Errorf("prompt %s has no version %d", name, version)
}
