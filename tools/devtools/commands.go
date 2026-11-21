package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// install puts every Go tool at its latest version, points git at the repo's
// hooks, fetches the desktop app's packages, then reports like doctor.
func (a *app) install(ctx context.Context) int {
	gi, res := a.checkGo(ctx)
	if res.st == fail {
		return a.report([]result{res})
	}
	a.installTools(ctx, gi)
	a.say("setting git hooks to %s", hooksDir)
	if err := a.setHooks(ctx); err != nil {
		a.say("  %v", err)
	}
	if _, err := a.r.lookPath(flutter); err == nil {
		a.say("fetching desktop app packages (exact versions from pubspec.lock)")
		args := []string{"pub", "get", "--enforce-lockfile"}
		c := cmd{dir: filepath.Join(a.root, "desktop"), name: flutter, args: args, stream: true}
		if _, err := a.exec(ctx, c); err != nil {
			a.say("  %v", err)
		}
	}
	return a.doctor(ctx, false)
}

// update moves go.mod to the latest Go toolchain, reinstalls every Go tool
// with it and upgrades Flutter, then reports like doctor.
func (a *app) update(ctx context.Context) int {
	if _, res := a.checkGo(ctx); res.st == fail {
		return a.report([]result{res})
	}
	a.say("moving go.mod to the latest Go toolchain")
	for _, args := range [][]string{{"get", "toolchain@latest"}, {"mod", "tidy"}} {
		if _, err := a.exec(ctx, cmd{dir: a.root, name: "go", args: args, stream: true}); err != nil {
			a.say("  %v", err)
			return 1
		}
	}
	gi, res := a.checkGo(ctx) // downloads the new toolchain
	if res.st == fail {
		return a.report([]result{res})
	}
	a.say("Go toolchain: %s", gi.version)
	a.installTools(ctx, gi)
	if _, err := a.r.lookPath(flutter); err == nil {
		a.say("upgrading Flutter")
		if _, err := a.exec(ctx, cmd{dir: a.root, name: flutter, args: []string{"upgrade"}, stream: true}); err != nil {
			a.say("  %v", err)
		}
	}
	return a.doctor(ctx, false)
}

// doctor checks everything; with fix it reinstalls broken Go tools and resets
// the git hooks path. It returns 1 when a required item is still failing.
func (a *app) doctor(ctx context.Context, fix bool) int {
	gi, res := a.checkGo(ctx)
	results := []result{res}
	if res.st != fail {
		results = append(results, a.checkPath(gi.bin), a.checkRace(ctx))
		for _, t := range goTools {
			results = append(results, a.toolResult(ctx, gi, t, fix))
		}
	}
	hooks := a.checkHooks(ctx)
	if hooks.st == fail && fix && hooks.detail != "git not installed" {
		a.say("repairing git hooks: %s", hooks.detail)
		if err := a.setHooks(ctx); err == nil {
			hooks = a.checkHooks(ctx)
		}
	}
	results = append(results, hooks, a.checkFlutter(ctx))
	return a.report(results)
}

func (a *app) toolResult(ctx context.Context, gi goInfo, t tool, fix bool) result {
	res := a.checkTool(ctx, gi, t)
	if res.st == ok || !fix {
		return res
	}
	a.say("repairing %s: %s", t.name, res.detail)
	if err := a.goInstall(ctx, gi, t); err != nil {
		res.detail = fmt.Sprintf("%s; install failed: %s", res.detail, firstLine(err.Error()))
		return res
	}
	after := a.checkTool(ctx, gi, t)
	if after.st == ok {
		after.detail += " (repaired)"
	}
	return after
}

func (a *app) installTools(ctx context.Context, gi goInfo) {
	for _, t := range goTools {
		a.say("installing %s@latest", t.name)
		if err := a.goInstall(ctx, gi, t); err != nil {
			a.say("  %v", err)
		}
	}
}

// goInstall builds a tool with at least the repo's toolchain, so linters can
// load code that uses the newest language features.
func (a *app) goInstall(ctx context.Context, gi goInfo, t tool) error {
	_, err := a.exec(ctx, cmd{
		dir:    a.root,
		env:    []string{"GOTOOLCHAIN=" + gi.version + "+auto"},
		name:   "go",
		args:   []string{"install", t.pkg + "@latest"},
		stream: true,
	})
	return err
}

func (a *app) report(results []result) int {
	a.say("")
	code := 0
	for _, r := range results {
		a.say("%-5s %-14s %s", r.st, r.name, r.detail)
		if r.hint != "" && r.st != ok {
			for i, line := range strings.Split(r.hint, "\n") {
				mark := "->"
				if i > 0 {
					mark = "  "
				}
				a.say("%-20s%s %s", "", mark, line)
			}
		}
		if r.st == fail {
			code = 1
		}
	}
	if code == 0 {
		a.say("\nall required tools are ready")
	} else {
		a.say("\nsome required tools need attention (see above)")
	}
	return code
}
