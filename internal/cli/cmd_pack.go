package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/pack"
	"github.com/samautomateai/tessivo/internal/service"
	"github.com/samautomateai/tessivo/internal/template"
)

const subRefresh = "refresh"

// cmdPack manages team packs: tess pack add|list|refresh|remove|init|export.
func cmdPack(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("pack: want add|list|refresh|remove|init|export")
	}
	switch args[0] {
	case "init":
		return packInit(args[1:], env)
	case "export":
		return packExport(args[1:], env)
	case subAdd, subList, subRefresh, subRemove:
	default:
		return usagef("pack: unknown subcommand %q", args[0])
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	msg, err := packRun(ctx, svc, args[0], args[1:])
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(env.Stdout, msg)
	return wrapWrite(err)
}

// packRun runs add, remove, refresh or list and returns what to print.
func packRun(ctx context.Context, svc *service.Service, sub string, rest []string) (string, error) {
	switch sub {
	case subAdd:
		if len(rest) < 1 || len(rest) > 2 {
			return "", usagef("pack add: want <folder|file.zip|git URL> [branch]")
		}
		branch := ""
		if len(rest) == 2 {
			branch = rest[1]
		}
		info, err := svc.AddPack(ctx, rest[0], branch)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("added %q (%s): %d templates, %d prompts, %d MCP servers (off until trusted)\n",
			info.Name, info.ID, info.Templates, info.Prompts, info.Servers), nil
	case subRemove:
		if len(rest) != 1 {
			return "", usagef("pack remove: want <pack id>")
		}
		if err := svc.RemovePack(rest[0]); err != nil {
			return "", err
		}
		return "removed " + rest[0] + "\n", nil
	case subRefresh:
		return packTable(svc.RefreshPacks(ctx)), nil
	}
	return packTable(svc.Packs()), nil
}

// packTable lists packs one per line: id, kind, name, item counts, problems.
func packTable(list []service.PackInfo) string {
	var b strings.Builder
	for _, p := range list {
		fmt.Fprintf(&b, "%-20s %-7s %-28s %d templates, %d prompts, %d servers", p.ID, p.Kind, p.Name, p.Templates,
			p.Prompts, p.Servers)
		if p.Error != "" {
			fmt.Fprintf(&b, "  (%s)", p.Error)
		}
		if len(p.Problems) > 0 {
			fmt.Fprintf(&b, "  %d problem(s)", len(p.Problems))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// packInit writes an empty pack: tess pack init <dir> --id x --name "…".
func packInit(args []string, env Env) error {
	fs := newFlags("pack init", env)
	id := fs.String("id", "", "pack id (a-z, 0-9, '-')")
	name := fs.String("name", "", "pack name")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *id == "" || *name == "" {
		return usagef("pack init: want <dir> --id <id> --name <name>")
	}
	if err := pack.Init(pos[0], *id, *name); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "wrote an empty pack in %s\n", pos[0])
	return wrapWrite(err)
}

// packExport writes chosen user templates, a notebook's saved prompts and
// configured MCP servers into a pack folder.
func packExport(args []string, env Env) error {
	fs := newFlags("pack export", env)
	id := fs.String("id", "", "pack id (a-z, 0-9, '-')")
	name := fs.String("name", "", "pack name")
	tpls := fs.String("templates", "", "user template IDs, comma-separated")
	promptsFrom := fs.String("prompts", "", "a notebook folder whose saved prompts to include")
	servers := fs.String("mcp", "", "configured MCP server names, comma-separated")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *id == "" || *name == "" {
		return usagef("pack export: want <dir> --id <id> --name <name> [--templates a,b] [--prompts <notebook>] " +
			"[--mcp a,b]")
	}
	ts, err := exportTemplates(env, *tpls)
	if err != nil {
		return err
	}
	ps, err := exportPrompts(*promptsFrom)
	if err != nil {
		return err
	}
	ss, err := exportServers(env, *servers)
	if err != nil {
		return err
	}
	if err := pack.Export(pos[0], pack.Manifest{ID: *id, Name: *name}, ts, ps, ss); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "wrote %d templates, %d prompts and %d MCP servers to %s\n",
		len(ts), len(ps), len(ss), pos[0])
	return wrapWrite(err)
}

func split(list string) []string {
	var out []string
	for _, s := range strings.Split(list, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func exportTemplates(env Env, ids string) ([]template.Template, error) {
	lib, err := library(env)
	if err != nil {
		return nil, err
	}
	var out []template.Template
	for _, id := range split(ids) {
		t, err := lib.Get(id)
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", id, err)
		}
		out = append(out, t)
	}
	return out, nil
}

func exportPrompts(dir string) ([]pack.Prompt, error) {
	if dir == "" {
		return nil, nil
	}
	nb, err := notebook.Open(dir)
	if err != nil {
		return nil, err
	}
	names, err := nb.ListPrompts()
	if err != nil {
		return nil, err
	}
	var out []pack.Prompt
	for _, n := range names {
		p, err := nb.LoadPrompt(n)
		if err != nil {
			return nil, err
		}
		out = append(out, pack.Prompt{Name: p.Name, Text: p.Text, WorkType: nb.Manifest.WorkType, Settings: p.Settings})
	}
	return out, nil
}

func exportServers(env Env, names string) ([]config.MCPServer, error) {
	if names == "" {
		return nil, nil
	}
	f, err := loadConfig(env)
	if err != nil {
		return nil, err
	}
	var out []config.MCPServer
	for _, n := range split(names) {
		s, ok := f.MCP.Find(n)
		if !ok {
			return nil, fmt.Errorf("mcp server %q is not configured", n)
		}
		out = append(out, s)
	}
	return out, nil
}
