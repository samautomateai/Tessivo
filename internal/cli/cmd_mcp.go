package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

const (
	subTrust   = "trust"
	subUntrust = "untrust"
)

const mcpHelp = `tess mcp add <name> (--command PROG [--arg A]... | --url URL) [--env K=V]... [--header K=V]...
    [--allow-insecure-http]     values may hold ${secret:NAME}; credentials are refused as text
tess mcp list | test <name> | remove <name>
tess mcp attach|detach <notebook> <name>   let the notebook's agent runs use the server's tools
tess mcp trust|untrust <pack>/<server>     let a server from a team pack run (shows what it runs)`

func cmdMCP(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("mcp: want a subcommand\n%s", mcpHelp)
	}
	sub, rest := args[0], args[1:]
	run, ok := map[string]func(context.Context, *service.Service, []string, Env) error{
		subAdd: mcpAdd, subList: mcpList, subTest: mcpTest, subRemove: mcpRemove,
		subAttach: mcpAttach, subDetach: mcpAttach, subTrust: mcpTrust, subUntrust: mcpTrust,
	}[sub]
	if !ok {
		return usagef("mcp: unknown subcommand %q\n%s", sub, mcpHelp)
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	if sub == subAttach || sub == subDetach || sub == subTrust || sub == subUntrust {
		rest = append([]string{sub}, rest...)
	}
	return cliErr(run(ctx, svc, rest, env))
}

func mcpAdd(_ context.Context, svc *service.Service, args []string, env Env) error {
	fs := newFlags("mcp add", env)
	fs.Usage = func() { _, _ = fmt.Fprintln(env.Stderr, mcpHelp) }
	var srv config.MCPServer
	var argv, envs, headers multiFlag
	fs.StringVar(&srv.Command, "command", "", "program that runs the server (stdio)")
	fs.Var(&argv, "arg", "argument for the program (repeatable)")
	fs.StringVar(&srv.URL, "url", "", "Streamable HTTP endpoint")
	fs.Var(&envs, "env", "K=V for the program's environment (repeatable)")
	fs.Var(&headers, "header", "K=V HTTP header (repeatable)")
	fs.BoolVar(&srv.AllowInsecureHTTP, "allow-insecure-http", false, "allow plain http to a non-loopback host")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("mcp add: want <name>\n%s", mcpHelp)
	}
	srv.Name, srv.Args = pos[0], argv
	if srv.Env, err = keyValues("--env", envs); err != nil {
		return err
	}
	if srv.Headers, err = keyValues("--header", headers); err != nil {
		return err
	}
	if _, err := svc.AddMCPServer(srv); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "added %s; check it with: tess mcp test %s\n", srv.Name, srv.Name)
	return wrapWrite(err)
}

func keyValues(flagName string, items []string) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := map[string]string{}
	for _, kv := range items {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, usagef("mcp add: %s wants KEY=VALUE, got %q", flagName, k)
		}
		out[k] = v
	}
	return out, nil
}

func mcpList(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 0 {
		return usagef("mcp list: no arguments")
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tTRANSPORT\tTARGET\tTRUST")
	for _, s := range svc.MCPServers() {
		transport, target := "stdio", strings.TrimSpace(s.Command+" "+strings.Join(s.Args, " "))
		if s.URL != "" {
			transport, target = "http", s.URL
		}
		trust := "yours"
		switch {
		case s.Pack != "" && s.Trusted:
			trust = "trusted"
		case s.Pack != "":
			trust = "not trusted (tess mcp trust " + s.Name + ")"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.Name, transport, target, trust)
	}
	return wrapWrite(tw.Flush())
}

func mcpTest(ctx context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 1 {
		return usagef("mcp test: want <name>")
	}
	res, err := svc.TestMCPServer(ctx, args[0])
	if err != nil {
		return err
	}
	var sb strings.Builder
	section := func(n int, noun string, lines []string) {
		_, _ = fmt.Fprintf(&sb, "%d %s\n", n, plural(n, noun))
		for _, l := range lines {
			sb.WriteString("  " + l + "\n")
		}
	}
	var tools, resources, prompts []string
	for _, t := range res.Tools {
		tools = append(tools, oneLine(strings.TrimSpace(t.Name+"  "+t.Description), 100))
	}
	for _, r := range res.Resources {
		resources = append(resources, oneLine(strings.TrimSpace(r.URI+"  "+r.Name), 100))
	}
	for _, p := range res.Prompts {
		prompts = append(prompts, oneLine(strings.TrimSpace(p.Name+"  "+p.Description), 100))
	}
	section(len(res.Tools), "tool", tools)
	section(len(res.Resources), "resource", resources)
	section(len(res.Prompts), "prompt", prompts)
	_, err = fmt.Fprint(env.Stdout, sb.String())
	return wrapWrite(err)
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

func mcpRemove(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 1 {
		return usagef("mcp remove: want <name>")
	}
	if err := svc.RemoveMCPServer(args[0]); err != nil {
		return err
	}
	_, err := fmt.Fprintln(env.Stdout, "removed "+args[0])
	return wrapWrite(err)
}

// mcpAttach handles attach and detach; args[0] says which.
func mcpAttach(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 3 {
		return usagef("mcp %s: want <notebook> <name>", args[0])
	}
	dir, err := filepath.Abs(args[1])
	if err != nil {
		return fmt.Errorf("notebook %s: %w", args[1], err)
	}
	nb, err := notebook.Open(dir)
	if err != nil {
		return err
	}
	name, list := args[2], slices.Clone(nb.Manifest.Agent.MCPServers)
	if args[0] == subAttach {
		list = append(list, name)
	} else {
		list = slices.DeleteFunc(list, func(s string) bool { return s == name })
	}
	svc.NotebookDirs = []string{dir}
	if _, err := svc.SetNotebookMCP(dir, list); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "%sed %s\n", args[0], name)
	return wrapWrite(err)
}

// Attach and detach subcommand words.
const (
	subAttach = "attach"
	subDetach = "detach"
)

// mcpTrust trusts or stops trusting a pack server, showing exactly what runs:
// tess mcp trust|untrust <pack>/<server>.
func mcpTrust(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 2 {
		return usagef("mcp %s: want <pack>/<server>", args[0])
	}
	trust, name := args[0] == subTrust, args[1]
	if err := svc.TrustMCP(name, trust); err != nil {
		return err
	}
	if !trust {
		_, err := fmt.Fprintf(env.Stdout, "%s is no longer trusted\n", name)
		return wrapWrite(err)
	}
	for _, s := range svc.MCPServers() {
		if s.Name == name {
			what := s.URL
			if what == "" {
				what = strings.TrimSpace(s.Command + " " + strings.Join(s.Args, " "))
			}
			_, err := fmt.Fprintf(env.Stdout, "trusted %s: %s\n", name, what)
			return wrapWrite(err)
		}
	}
	return nil
}
