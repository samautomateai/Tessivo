package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
	"github.com/samautomateai/tessivo/internal/template"
)

func templatesDir(env Env) (string, error) {
	p, err := configPath(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "templates"), nil
}

func library(env Env) (template.Library, error) {
	dir, err := templatesDir(env)
	return template.Library{Dir: dir}, err
}

func cmdTemplate(args []string, env Env) error {
	if len(args) == 0 {
		return usagef("template: want list|show|use|save|extend|delete")
	}
	lib, err := library(env)
	if err != nil {
		return err
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case subList:
		return templateList(lib, rest, env)
	case subShow:
		return templateShow(lib, rest, env)
	case "use":
		return templateUse(lib, rest, env)
	case subSave:
		return templateSave(lib, rest, env)
	case "extend":
		return templateExtend(lib, rest, env)
	case subDelete:
		if len(rest) != 1 {
			return usagef("template delete: want <id>")
		}
		return lib.Delete(rest[0])
	default:
		return usagef("template: unknown subcommand %q", sub)
	}
}

func templateList(lib template.Library, args []string, env Env) error {
	fs := newFlags("template list", env)
	wt := fs.String("type", "", "only this work type")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	list, err := lib.List()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTYPE\tSOURCE\tVERSION\tNAME\tNOTE")
	for _, t := range list {
		if *wt != "" && t.WorkType != *wt {
			continue
		}
		note := ""
		if n, changed, _ := lib.ParentUpdate(t); n > 0 {
			note = fmt.Sprintf("parent updated to v%d (%s)", n, strings.Join(changed, ", "))
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\tv%d\t%s\t%s\n", t.ID, t.WorkType, t.Source, t.Version, t.Name, note)
	}
	return wrapWrite(tw.Flush())
}

func templateShow(lib template.Library, args []string, env Env) error {
	if len(args) != 1 {
		return usagef("template show: want <id>")
	}
	t, err := lib.Get(args[0])
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(t, "", "  ")
	_, err = fmt.Fprintln(env.Stdout, string(b))
	return wrapWrite(err)
}

func templateUse(lib template.Library, args []string, env Env) error {
	fs := newFlags("template use", env)
	name := fs.String("name", "", "notebook name (default: the template's name)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 2 {
		return usagef("template use: want <template-id> <dir>")
	}
	t, err := lib.Get(pos[0])
	if err != nil {
		return err
	}
	if *name == "" {
		*name = t.Name
	}
	nb, err := notebook.Create(pos[1], *name, t.WorkType)
	if err != nil {
		return err
	}
	if err := template.Apply(nb, t); err != nil {
		return err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "created %q from %s v%d in %s\nadd context:\n", *name, t.ID, t.Version, nb.Dir)
	for _, s := range t.Slots {
		req := "optional"
		if s.Required {
			req = "required"
		}
		fmt.Fprintf(&sb, "  - %s (%s, %s): %s\n", s.Name, s.Kind, req, s.Description)
	}
	if t.Schedule != nil {
		sb.WriteString("a schedule was added disabled; enable it with 'tess schedule enable' after adding context\n")
	}
	_, err = fmt.Fprint(env.Stdout, sb.String())
	return wrapWrite(err)
}

func templateSave(lib template.Library, args []string, env Env) error {
	fs := newFlags("template save", env)
	id := fs.String("id", "", "template id (default: from the name)")
	name := fs.String("name", "", "template name")
	desc := fs.String("description", "", "what the template is for")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *name == "" {
		return usagef("template save: want <notebook> --name NAME")
	}
	nb, err := notebook.Open(pos[0])
	if err != nil {
		return err
	}
	if *id == "" {
		*id = service.Slug(*name)
	}
	t, err := template.FromNotebook(nb, *id, *name, *desc)
	if err != nil {
		return err
	}
	saved, err := lib.Save(t)
	if err != nil {
		return usageError{err.Error()}
	}
	_, err = fmt.Fprintf(env.Stdout, "saved template %s v%d (context data is never included)\n",
		saved.ID, saved.Version)
	return wrapWrite(err)
}

func templateExtend(lib template.Library, args []string, env Env) error {
	fs := newFlags("template extend", env)
	id := fs.String("id", "", "new template id (default: from the name)")
	name := fs.String("name", "", "new template name")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *name == "" {
		return usagef("template extend: want <parent-id> --name NAME")
	}
	parent, err := lib.Get(pos[0])
	if err != nil {
		return err
	}
	if *id == "" {
		*id = service.Slug(*name)
	}
	saved, err := lib.Save(template.Extend(parent, *id, *name))
	if err != nil {
		return usageError{err.Error()}
	}
	_, err = fmt.Fprintf(env.Stdout, "created %s from %s; edit %s to customise\n", saved.ID, saved.Parent,
		filepath.Join(lib.Dir, saved.ID+".json"))
	return wrapWrite(err)
}

func cmdMemory(args []string, env Env) error {
	if len(args) < 2 {
		return usagef("memory: want list|add|remove <notebook> [text|number]")
	}
	nb, err := notebook.Open(args[1])
	if err != nil {
		return err
	}
	mem := nb.Manifest.Memory
	switch args[0] {
	case subList:
		for i, m := range mem {
			if _, err := fmt.Fprintf(env.Stdout, "%d. %s\n", i+1, m); err != nil {
				return wrapWrite(err)
			}
		}
		return nil
	case subAdd:
		text := strings.TrimSpace(strings.Join(args[2:], " "))
		if text == "" || len(text) > service.MaxMemoryItem || len(mem) >= template.MaxMemory {
			return usagef("memory add: want 1-%d characters and at most %d items", service.MaxMemoryItem,
				template.MaxMemory)
		}
		next := make([]string, 0, len(mem)+1)
		nb.Manifest.Memory = append(append(next, mem...), text)
	case "remove":
		n, err := strconv.Atoi(strings.Join(args[2:], ""))
		if err != nil || n < 1 || n > len(mem) {
			return usagef("memory remove: want an item number from 'tess memory list'")
		}
		next := make([]string, 0, len(mem)-1)
		nb.Manifest.Memory = append(append(next, mem[:n-1]...), mem[n:]...)
	default:
		return usagef("memory: unknown subcommand %q", args[0])
	}
	return nb.Save()
}
