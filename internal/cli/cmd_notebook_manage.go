package cli

import (
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/notebook"
)

const (
	subArchive = "archive"
	subPin     = "pin"
	subUnpin   = "unpin"
)

// notebookSubs are the notebook subcommands with their own argument handling.
var notebookSubs = map[string]func([]string, Env) error{
	"import":   notebookImport,
	"list":     notebookList,
	"commands": notebookCommands,
	"export":   notebookExport,
	"tag":      notebookTag,
	"delete":   notebookDelete,
	"watch":    notebookWatch,
	"worktype": notebookWorkType,
	subPin:     func(a []string, env Env) error { return notebookPin(a, true, env) },
	subUnpin:   func(a []string, env Env) error { return notebookPin(a, false, env) },
}

// cmdNotebook renames, archives, restores or deletes a notebook folder.
func cmdNotebook(args []string, env Env) error {
	if len(args) > 0 {
		if run, ok := notebookSubs[args[0]]; ok {
			return run(args[1:], env)
		}
	}
	if len(args) < 2 {
		return usagef("notebook: want list, import, or rename|archive|unarchive|pin|unpin|tag|watch|export|delete|" +
			"commands <notebook> ...")
	}
	sub := args[0]
	nb, err := notebook.Open(args[1])
	if err != nil {
		return err
	}
	var msg string
	switch sub {
	case "rename":
		if len(args) < 3 {
			return usagef("notebook rename: want <notebook> <new name>")
		}
		err = nb.Rename(strings.Join(args[2:], " "))
		msg = fmt.Sprintf("renamed to %q", nb.Manifest.Name)
	case subArchive, "unarchive":
		err = nb.SetArchived(sub == subArchive)
		msg = sub + "d " + nb.Manifest.Name
	default:
		return usagef("notebook: unknown subcommand %q", sub)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, msg)
	return wrapWrite(err)
}

func notebookDelete(args []string, env Env) error {
	fs := newFlags("notebook delete", env)
	yes := fs.Bool("yes", false, "confirm: delete the notebook folder and everything in it")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || !*yes {
		return usagef("notebook delete: want <notebook> and --yes (deletes the folder and everything in it)")
	}
	if err := notebook.Remove(pos[0]); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "deleted %s\n", pos[0])
	return wrapWrite(err)
}
