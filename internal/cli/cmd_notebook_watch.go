package cli

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

// notebookWatch sets what tess serve does when the notebook's folders change:
// tess notebook watch <notebook> off|notify|run.
func notebookWatch(args []string, env Env) error {
	if len(args) != 2 {
		return usagef("notebook watch: want <notebook> off|notify|run")
	}
	nb, err := notebook.Open(args[0])
	if err != nil {
		return err
	}
	if err := nb.SetWatch(args[1]); err != nil {
		return usagef("notebook watch: %v", err)
	}
	_, err = fmt.Fprintf(env.Stdout, "%s: watch %s (checked while tess serve runs)\n", nb.Manifest.Name, args[1])
	return wrapWrite(err)
}

// notebookWorkType shows the work type the sources suggest, or sets one:
// tess notebook worktype <notebook> [analyze|summarize|document|report].
func notebookWorkType(args []string, env Env) error {
	if len(args) != 1 && len(args) != 2 {
		return usagef("notebook worktype: want <notebook> [work type]")
	}
	nb, err := notebook.Open(args[0])
	if err != nil {
		return err
	}
	if len(args) == 2 {
		if err := service.ChangeWorkType(nb, args[1]); err != nil {
			return err
		}
		_, err = fmt.Fprintf(env.Stdout, "%s: work type %s\n", nb.Manifest.Name, args[1])
		return wrapWrite(err)
	}
	msg := fmt.Sprintf("%s: %s", nb.Manifest.Name, nb.Manifest.WorkType)
	if sug := service.SuggestFor(nb); sug.WorkType != "" {
		msg += fmt.Sprintf(" (suggested: %s, because %s; set it with: tess notebook worktype %s %s)",
			sug.WorkType, sug.Reason, args[0], sug.WorkType)
	}
	_, err = fmt.Fprintln(env.Stdout, msg)
	return wrapWrite(err)
}
