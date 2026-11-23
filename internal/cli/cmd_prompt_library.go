package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/worktype"
)

// promptLibrary lists the starter prompts (`tess prompt library [--type T]`)
// or prints one's text (`tess prompt library show <id>`), ready to pipe into
// `tess prompt save <notebook> <name>`.
func promptLibrary(args []string, env Env) error {
	fs := newFlags("prompt library", env)
	wt := fs.String("type", "", "only prompts for this work type")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 2 && pos[0] == subShow {
		for _, p := range worktype.Library {
			if p.ID == pos[1] {
				_, err := fmt.Fprintln(env.Stdout, p.Text)
				return wrapWrite(err)
			}
		}
		return usagef("prompt library: no prompt %q (tess prompt library lists them)", pos[1])
	}
	if len(pos) != 0 {
		return usagef("prompt library: want [--type T] or show <id>")
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTYPE\tNAME")
	for _, p := range worktype.LibraryFor(*wt) {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", p.ID, p.WorkType, p.Name)
	}
	return wrapWrite(tw.Flush())
}
