package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/service"
)

// toolLine prints chat tool activity to stderr.
func toolLine(env Env) func(service.ChatToolEvent) {
	return func(e service.ChatToolEvent) {
		switch e.Kind {
		case "call":
			_, _ = fmt.Fprintf(env.Stderr, "\n[tool] %s %s\n", e.Tool, e.Args)
		case "result":
			_, _ = fmt.Fprintf(env.Stderr, "[tool] %s → %s\n", e.Tool, e.Summary)
		default:
			_, _ = fmt.Fprintf(env.Stderr, "[tool] %s %s\n", e.Tool, e.Kind)
		}
	}
}

// ttyLines reads answers from a terminal; nil when stdin is not one.
func ttyLines(env Env) func() (string, bool) {
	if !env.StdinIsTTY || env.Stdin == nil {
		return nil
	}
	r := bufio.NewReader(env.Stdin)
	return func() (string, bool) {
		line, err := r.ReadString('\n')
		return line, err == nil || line != ""
	}
}

// askTool asks on the terminal whether a tool may run: y (once), a (always
// in this chat), n (no) or d (never in this chat). Without a terminal it
// denies and says how to allow the tool.
func askTool(env Env, readLine func() (string, bool)) func(service.ChatToolRequest) service.ChatToolAnswer {
	return func(req service.ChatToolRequest) service.ChatToolAnswer {
		if readLine == nil {
			_, _ = fmt.Fprintf(env.Stderr, "[tool] %s needs approval; allow it with an agent tool rule "+
				"(agent.tools in the config) or chat interactively\n", req.Tool)
			return service.ChatToolAnswer{}
		}
		_, _ = fmt.Fprintf(env.Stderr, "Allow %s %s? [y]es, [a]lways, [n]o, [d]eny always: ", req.Tool, req.Args)
		line, _ := readLine()
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return service.ChatToolAnswer{Allow: true}
		case "a", "always":
			return service.ChatToolAnswer{Allow: true, Remember: true}
		case "d":
			return service.ChatToolAnswer{Remember: true}
		default:
			return service.ChatToolAnswer{}
		}
	}
}
