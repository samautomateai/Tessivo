package cli

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/notebook"
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
