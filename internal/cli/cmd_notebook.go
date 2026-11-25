package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
)

func cmdNew(args []string, env Env) error {
	fs := newFlags("new", env)
	wt := fs.String("type", "", "work type: analyze, summarize, document or report")
	name := fs.String("name", "", "display name (default: folder name)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *wt == "" {
		return usagef("new: want <dir> and --type")
	}
	nb, err := notebook.Create(pos[0], *name, *wt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "created %s notebook %q in %s\n", nb.Manifest.WorkType, nb.Manifest.Name, nb.Dir)
	return wrapWrite(err)
}

func cmdContext(args []string, env Env) error {
	if len(args) < 2 {
		return usagef("context: want add|list|enable|disable <notebook> ...")
	}
	if args[0] == "relink" {
		return contextRelink(args[1:], env)
	}
	sub, dir, rest := args[0], args[1], args[2:]
	nb, err := notebook.Open(dir)
	if err != nil {
		return err
	}
	switch sub {
	case subAdd:
		return contextAdd(nb, rest, env)
	case subList:
		return contextList(nb, env)
	case subEnable, "disable":
		if len(rest) != 1 {
			return usagef("context %s: want <notebook> <id-or-label>", sub)
		}
		return nb.SetEnabled(rest[0], sub == subEnable)
	default:
		return usagef("context: unknown subcommand %q", sub)
	}
}

func contextAdd(nb *notebook.Notebook, args []string, env Env) error {
	fs := newFlags("context add", env)
	text := fs.String("text", "", "add this text as a source (- reads stdin)")
	label := fs.String("label", "", "label for --text or --git-diff")
	gitRepo := fs.String("git-diff", "", "add a repository's changes against --base (read with git on every run)")
	base := fs.String("base", "main", "--git-diff: the branch, tag or commit to compare with")
	paths, err := parse(fs, args)
	if err != nil {
		return err
	}
	if *text != "" {
		return contextAddText(nb, *text, *label, env)
	}
	if *gitRepo != "" {
		item, err := nb.AddGitDiff(*gitRepo, *base, *label)
		if err != nil {
			return usageError{err.Error()}
		}
		_, err = fmt.Fprintln(env.Stdout, "added "+item.Label+" (git changes, read on every run) id="+item.ID)
		return wrapWrite(err)
	}
	if len(paths) == 0 {
		return usagef("context add: want at least one path, URL or --text")
	}
	for _, p := range paths {
		item, err := addContextItem(nb, p, env)
		if err != nil {
			return err
		}
		how := "copied"
		switch {
		case item.IsMCP():
			how = "MCP resource, read at run time"
		case item.IsURL():
			how = "web page, fetched on every run"
		case item.InPlace:
			how = "referenced in place"
		}
		if _, err := fmt.Fprintf(env.Stdout, "added %s (%s) id=%s\n", item.Label, how, item.ID); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}

func contextList(nb *notebook.Notebook, env Env) error {
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tLABEL\tON\tSOURCES\t~TOKENS")
	for _, c := range nb.Manifest.Context {
		docs, err := extract.Path(nb.ContextPath(c))
		n, tokens := 0, 0
		if err == nil {
			n = len(docs)
			for _, d := range docs {
				tokens += d.Tokens()
			}
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%v\t%d\t%d\n", c.ID, c.Label, c.Enabled, n, tokens)
	}
	return wrapWrite(tw.Flush())
}

func cmdOutputs(args []string, env Env) error {
	if len(args) != 1 {
		return usagef("outputs: want <notebook>")
	}
	nb, err := notebook.Open(args[0])
	if err != nil {
		return err
	}
	outs, err := nb.ListOutputs()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCREATED\tTYPE\tTITLE")
	for _, o := range outs {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", o.ID, o.CreatedAt, o.WorkType, o.Title)
	}
	return wrapWrite(tw.Flush())
}

// addContextItem adds a file or folder, or an MCP resource written as
// mcp://<server>/<uri> (the server must be configured).
func addContextItem(nb *notebook.Notebook, p string, env Env) (notebook.ContextItem, error) {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		item, err := nb.AddURL(p, "")
		if err != nil {
			return notebook.ContextItem{}, usageError{err.Error()}
		}
		return item, nil
	}
	rest, ok := strings.CutPrefix(p, "mcp://")
	if !ok {
		return nb.AddContext(p)
	}
	server, uri, _ := strings.Cut(rest, "/")
	if server == "" || uri == "" {
		return notebook.ContextItem{}, usagef("context add: want mcp://<server>/<uri>")
	}
	f, err := loadConfig(env)
	if err != nil {
		return notebook.ContextItem{}, err
	}
	if _, ok := f.MCP.Find(server); !ok {
		return notebook.ContextItem{}, usagef("context add: mcp server %q is not configured (tess mcp add)", server)
	}
	item, err := nb.AddMCPResource(server, uri, "")
	if err != nil {
		return notebook.ContextItem{}, usageError{err.Error()}
	}
	return item, nil
}

// contextAddText saves text (or stdin for "-") as a Markdown source.
func contextAddText(nb *notebook.Notebook, text, label string, env Env) error {
	if text == "-" {
		if env.Stdin == nil || env.StdinIsTTY {
			return usagef("context add --text -: pipe the text in")
		}
		b, err := io.ReadAll(io.LimitReader(env.Stdin, notebook.MaxTextSource+1))
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		text = string(b)
	}
	item, err := nb.AddText(label, text)
	if err != nil {
		return usageError{err.Error()}
	}
	_, err = fmt.Fprintf(env.Stdout, "added %s (text) id=%s\n", item.Label, item.ID)
	return wrapWrite(err)
}
