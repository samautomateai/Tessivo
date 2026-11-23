package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// cmdSearch searches the workspace's notebooks, outputs, prompts and research notes.
func cmdSearch(args []string, env Env) error {
	fs := newFlags("search", env)
	limit := fs.Int("limit", 20, "most results to show")
	words, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return usagef("search: want words to search for")
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	res, err := svc.Search(strings.Join(words, " "), *limit)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "KIND\tNOTEBOOK\tTITLE\tMATCH")
	for _, r := range res {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Kind, r.NotebookID, r.Title, r.Snippet)
	}
	return wrapWrite(tw.Flush())
}
