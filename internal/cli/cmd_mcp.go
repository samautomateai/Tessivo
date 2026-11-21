package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/service"
)

const mcpHelp = `tess mcp add <name> (--command PROG [--arg A]... | --url URL) [--env K=V]... [--header K=V]...
    [--allow-insecure-http]     values may hold ${secret:NAME}; credentials are refused as text
tess mcp list | test <name> | remove <name>`

func cmdMCP(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("mcp: want a subcommand\n%s", mcpHelp)
	}
	sub, rest := args[0], args[1:]
	run, ok := map[string]func(context.Context, *service.Service, []string, Env) error{
		subAdd: mcpAdd, subList: mcpList, "test": mcpTest, subRemove: mcpRemove,
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
	_, _ = fmt.Fprintln(tw, "NAME\tTRANSPORT\tTARGET")
	for _, s := range svc.MCPServers() {
		transport, target := "stdio", strings.TrimSpace(s.Command+" "+strings.Join(s.Args, " "))
		if s.URL != "" {
			transport, target = "http", s.URL
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, transport, target)
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
