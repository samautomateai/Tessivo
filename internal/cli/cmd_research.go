package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/research"
	"github.com/samautomateai/tessivo/internal/service"
)

const researchHelp = `tess research new <name> [--description D] | list | show <id> | delete <id>
tess research source add <id> <path|mcp://server/uri> [--no-summary] | list <id>
tess research source level <id> <source-id> full|summary|off | remove|refresh <id> <source-id>
tess research ask <id> <question> [--chat ID]
tess research note add <id> --title T <text> | list <id> | rm <id> <note-id>`

type researchCmd func(context.Context, *service.Service, []string, Env) error

func cmdResearch(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("research: want a subcommand\n%s", researchHelp)
	}
	cmds := map[string]researchCmd{
		"new": researchNew, subList: researchList, subShow: researchShow, subDelete: researchDelete,
		"source": researchSource, "ask": researchAsk, "note": researchNote,
	}
	run, ok := cmds[args[0]]
	if !ok {
		return usagef("research: unknown subcommand %q\n%s", args[0], researchHelp)
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	return cliErr(run(ctx, svc, args[1:], env))
}

func researchNew(_ context.Context, svc *service.Service, args []string, env Env) error {
	fs := newFlags("research new", env)
	desc := fs.String("description", "", "what the notebook is about (sent with every question)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usagef("research new: want <name>")
	}
	info, err := svc.CreateResearch(strings.Join(pos, " "), *desc)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "created research notebook %s\n", info.ID)
	return wrapWrite(err)
}

func researchList(_ context.Context, svc *service.Service, _ []string, env Env) error {
	list, err := svc.ListResearch()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tNAME\tSOURCES\tNOTES")
	for _, nb := range list {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%d\n", nb.ID, nb.Name, len(nb.Sources), nb.Notes)
	}
	return wrapWrite(tw.Flush())
}

func researchShow(ctx context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 1 {
		return usagef("research show: want <id>")
	}
	info, err := svc.GetResearch(ctx, args[0])
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(env.Stdout, "%s (%s)\n%s\n\n", info.Name, info.ID, info.Description)
	return printSources(env, info.Sources)
}

func researchDelete(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 1 {
		return usagef("research delete: want <id>")
	}
	if err := svc.DeleteResearch(args[0]); err != nil {
		return err
	}
	_, err := fmt.Fprintln(env.Stdout, "deleted "+args[0])
	return wrapWrite(err)
}

func printSources(env Env, sources []service.ResearchSourceInfo) error {
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tLABEL\tKIND\tLEVEL\tSUMMARY")
	for _, s := range sources {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Label, s.Kind, s.Level, s.Summary)
	}
	return wrapWrite(tw.Flush())
}

func researchSource(ctx context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) < 2 {
		return usagef("research source: want add|list|level|remove|refresh <id> ...\n%s", researchHelp)
	}
	sub, id, rest := args[0], args[1], args[2:]
	switch {
	case sub == subAdd:
		return researchSourceAdd(ctx, svc, id, rest, env)
	case sub == subList && len(rest) == 0:
		info, err := svc.GetResearch(ctx, id)
		if err != nil {
			return err
		}
		return printSources(env, info.Sources)
	case sub == "level" && len(rest) == 2:
		_, err := svc.SetSourceLevel(id, rest[0], rest[1])
		return err
	case sub == subRemove && len(rest) == 1:
		_, err := svc.RemoveResearchSource(id, rest[0])
		return err
	case sub == "refresh" && len(rest) == 1:
		return svc.RefreshSummary(ctx, id, rest[0], true)
	}
	return usagef("research source %s: wrong arguments\n%s", sub, researchHelp)
}

func researchSourceAdd(ctx context.Context, svc *service.Service, id string, args []string, env Env) error {
	fs := newFlags("research source add", env)
	noSummary := fs.Bool("no-summary", false, "add without waiting for the summary")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("research source add: want <id> <path|mcp://server/uri>")
	}
	path, server, uri := pos[0], "", ""
	if rest, ok := strings.CutPrefix(path, "mcp://"); ok {
		server, uri, _ = strings.Cut(rest, "/")
		path = ""
		if server == "" || uri == "" {
			return usagef("research source add: want mcp://<server>/<uri>")
		}
	} else if path, err = filepath.Abs(path); err != nil {
		return fmt.Errorf("source path: %w", err)
	}
	info, err := svc.AddResearchSource(ctx, id, path, server, uri, !*noSummary)
	if err != nil {
		return err
	}
	s := info.Sources[len(info.Sources)-1]
	_, err = fmt.Fprintf(env.Stdout, "added %s (%s); summary %s\n", s.Label, s.ID, s.Summary)
	return wrapWrite(err)
}

func researchAsk(ctx context.Context, svc *service.Service, args []string, env Env) error {
	fs := newFlags("research ask", env)
	chatID := fs.String("chat", "", "continue this conversation")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 2 {
		return usagef("research ask: want <id> <question>")
	}
	id, question := pos[0], strings.Join(pos[1:], " ")
	if *chatID == "" {
		sess, err := svc.CreateResearchChat(id, "")
		if err != nil {
			return err
		}
		*chatID = sess.ID
		_, _ = fmt.Fprintf(env.Stderr, "conversation %s\n", sess.ID)
	}
	streamed := false
	r, err := svc.SendResearch(ctx, id, *chatID, question, func(d string) {
		streamed = true
		_, _ = fmt.Fprint(env.Stdout, d)
	})
	if !streamed {
		_, _ = fmt.Fprint(env.Stdout, r.Assistant.Content)
	}
	_, _ = fmt.Fprintln(env.Stdout)
	if err != nil {
		return err
	}
	return printReferences(env, r.Citations)
}

func printReferences(env Env, cites []research.Citation) error {
	if len(cites) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("\nReferences:\n")
	for _, c := range cites {
		line := fmt.Sprintf("[%d] %s", c.N, c.Label)
		if c.Path != "" {
			line += ": " + c.Path
		}
		if c.Level != "" {
			line += " (" + c.Level + ")"
		}
		sb.WriteString(line + "\n")
	}
	_, err := fmt.Fprint(env.Stdout, sb.String())
	return wrapWrite(err)
}

func researchNote(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) < 2 {
		return usagef("research note: want add|list|rm <id> ...\n%s", researchHelp)
	}
	sub, id, rest := args[0], args[1], args[2:]
	switch {
	case sub == subAdd:
		fs := newFlags("research note add", env)
		title := fs.String("title", "", "note title")
		pos, err := parse(fs, rest)
		if err != nil {
			return err
		}
		if len(pos) == 0 {
			return usagef("research note add: want <id> <text>")
		}
		_, err = svc.SaveResearchNote(id, research.Note{Title: *title, Text: strings.Join(pos, " "), Include: true})
		return err
	case sub == subList && len(rest) == 0:
		notes, err := svc.ResearchNotes(id)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tTITLE\tORIGIN\tINCLUDED")
		for _, n := range notes {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%v\n", n.ID, n.Title, n.Origin, n.Include)
		}
		return wrapWrite(tw.Flush())
	case sub == "rm" && len(rest) == 1:
		return svc.DeleteResearchNote(id, rest[0])
	}
	return usagef("research note %s: wrong arguments\n%s", sub, researchHelp)
}
