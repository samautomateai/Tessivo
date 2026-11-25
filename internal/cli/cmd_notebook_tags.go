package cli

import (
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// notebookTag adds or removes one tag: tess notebook tag <notebook> add|remove <tag>.
func notebookTag(args []string, env Env) error {
	if len(args) < 3 || (args[1] != "add" && args[1] != "remove") {
		return usagef("notebook tag: want <notebook> add|remove <tag>")
	}
	nb, err := notebook.Open(args[0])
	if err != nil {
		return err
	}
	tag := strings.Join(args[2:], " ")
	next := []string{}
	for _, t := range nb.Manifest.Tags {
		if !strings.EqualFold(t, strings.TrimSpace(tag)) {
			next = append(next, t)
		}
	}
	if args[1] == "add" {
		next = append(next, tag)
	}
	if err := nb.SetTags(next); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "%s tags: %s\n", nb.Manifest.Name, strings.Join(nb.Manifest.Tags, ", "))
	return wrapWrite(err)
}

// notebookPin pins or unpins a notebook.
func notebookPin(args []string, pin bool, env Env) error {
	if len(args) != 1 {
		return usagef("notebook pin|unpin: want <notebook>")
	}
	path := args[0]
	nb, err := notebook.Open(path)
	if err != nil {
		return err
	}
	if err := nb.SetPinned(pin); err != nil {
		return err
	}
	word := "unpinned"
	if pin {
		word = "pinned"
	}
	_, err = fmt.Fprintf(env.Stdout, "%s %s\n", word, nb.Manifest.Name)
	return wrapWrite(err)
}

// notebookList lists the workspace notebooks, pinned first: tess notebook list [--tag t].
func notebookList(args []string, env Env) error {
	fs := newFlags("notebook list", env)
	tag := fs.String("tag", "", "only notebooks with this tag (any case)")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	list, err := svc.ListNotebooks()
	if *tag != "" {
		list, err = svc.ListNotebooksTagged(*tag)
	}
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, n := range list {
		mark := " "
		if n.Pinned {
			mark = "*"
		}
		fmt.Fprintf(&b, "%s %-24s %-10s %s", mark, n.ID, n.WorkType, n.Name)
		if len(n.Tags) > 0 {
			fmt.Fprintf(&b, "  [%s]", strings.Join(n.Tags, ", "))
		}
		b.WriteByte('\n')
	}
	_, err = fmt.Fprint(env.Stdout, b.String())
	return wrapWrite(err)
}
