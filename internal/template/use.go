package template

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// Apply sets up a freshly created notebook from t: settings, memory,
// prompts and the optional schedule. Context slots are left for the user.
func Apply(nb *notebook.Notebook, t Template) error {
	if nb.Manifest.WorkType != t.WorkType {
		return fmt.Errorf("notebook work type %s does not match template %s", nb.Manifest.WorkType, t.WorkType)
	}
	nb.Manifest.Settings = t.Settings
	nb.Manifest.Memory = append([]string(nil), t.Memory...)
	nb.Manifest.Template = t.ID + "@" + strconv.Itoa(t.Version)
	if err := nb.Save(); err != nil {
		return err
	}
	for _, p := range t.Prompts {
		if err := nb.SavePrompt(notebook.Prompt{Name: p.Name, Text: p.Text, Settings: p.Settings}); err != nil {
			return err
		}
	}
	if s := t.Schedule; s != nil {
		sch := notebook.Schedule{
			ID: notebook.NewID(), Cron: s.Cron, PromptName: s.PromptName, Missed: s.Missed,
			Enabled:   false, // the user enables it after adding context
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := nb.SaveSchedules([]notebook.Schedule{sch}); err != nil {
			return err
		}
	}
	return nil
}

// FromNotebook captures a notebook as a template: prompts, settings, memory
// and its first schedule. Context data is never copied; each context item
// becomes an optional slot describing what to add.
func FromNotebook(nb *notebook.Notebook, id, name, description string) (Template, error) {
	t := Template{
		ID: id, Name: name, Description: description, WorkType: nb.Manifest.WorkType,
		Settings: nb.Manifest.Settings, Memory: append([]string(nil), nb.Manifest.Memory...), Slots: []Slot{},
	}
	names, err := nb.ListPrompts()
	if err != nil {
		return Template{}, err
	}
	for _, n := range names {
		p, err := nb.LoadPrompt(n)
		if err != nil {
			return Template{}, err
		}
		t.Prompts = append(t.Prompts, Prompt{Name: p.Name, Text: p.Text, Settings: p.Settings})
	}
	if len(t.Prompts) == 0 {
		t.Prompts = []Prompt{{Name: "main", Text: "Run the " + t.WorkType + " work type on the context."}}
	}
	for _, c := range nb.Manifest.Context {
		kind := "documents"
		if c.InPlace {
			kind = "code base"
		}
		if len(t.Slots) < MaxSlots {
			t.Slots = append(t.Slots, Slot{Name: slotName(c.Label), Kind: kind, Description: "Like: " + c.Label})
		}
	}
	if list, err := nb.Schedules(); err == nil && len(list) > 0 {
		s := list[0]
		t.Schedule = &Schedule{Cron: s.Cron, PromptName: s.PromptName, Missed: s.Missed}
	}
	return t, nil
}

func slotName(label string) string {
	if i := strings.LastIndexByte(label, '.'); i > 0 {
		label = label[:i]
	}
	return strings.TrimSpace(label)
}

// Extend copies parent under a new id and name, recording its lineage.
func Extend(parent Template, id, name string) Template {
	t := parent
	t.ID, t.Name, t.Version, t.Source, t.UpdatedAt = id, name, 0, SourceUser, ""
	t.Parent = parent.ID + "@" + strconv.Itoa(parent.Version)
	t.Prompts = append([]Prompt(nil), parent.Prompts...)
	t.Slots = append([]Slot(nil), parent.Slots...)
	t.Memory = append([]string(nil), parent.Memory...)
	return t
}

// ParentUpdate reports whether t's parent has a newer version in the library,
// and which prompts changed, so users can review and pull changes in.
func (l Library) ParentUpdate(t Template) (newer int, changed []string, err error) {
	if t.Parent == "" {
		return 0, nil, nil
	}
	pid, ver, ok := strings.Cut(t.Parent, "@")
	have, convErr := strconv.Atoi(ver)
	if !ok || convErr != nil {
		return 0, nil, fmt.Errorf("bad parent reference %q", t.Parent)
	}
	parent, err := l.Get(pid)
	if err != nil {
		return 0, nil, err
	}
	if parent.Version <= have {
		return 0, nil, nil
	}
	mine := map[string]string{}
	for _, p := range t.Prompts {
		mine[p.Name] = p.Text
	}
	for _, p := range parent.Prompts {
		if mine[p.Name] != p.Text {
			changed = append(changed, p.Name)
		}
	}
	return parent.Version, changed, nil
}
