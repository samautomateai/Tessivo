// Command devtools installs, checks and upgrades the tools Tessivo development
// needs: the Go toolchain pinned in go.mod, Task, golangci-lint v2, govulncheck,
// gosec, the repo's git hooks and, for the desktop app, Flutter stable.
//
// It needs only Go, so it also bootstraps a fresh machine before Task exists:
//
//	go run ./tools/devtools install          # install every tool at its latest version, set git hooks
//	go run ./tools/devtools doctor [-check]  # diagnose the setup and repair what it can
//	go run ./tools/devtools update           # move go.mod to the latest Go toolchain, upgrade every tool
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"
)

// cmdTimeout bounds every external command; flutter upgrade is the slowest.
const cmdTimeout = 15 * time.Minute

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := mainCode(ctx)
	stop()
	os.Exit(code)
}

func mainCode(ctx context.Context) int {
	wd, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "devtools:", err)
		return 2
	}
	root, err := findRoot(wd)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "devtools:", err)
		return 2
	}
	a := &app{
		r:       osRunner{out: os.Stdout},
		out:     os.Stdout,
		root:    root,
		getenv:  os.Getenv,
		goos:    runtime.GOOS,
		timeout: cmdTimeout,
	}
	return run(ctx, a, os.Args[1:])
}

func run(ctx context.Context, a *app, args []string) int {
	if len(args) == 0 {
		usage(a.out)
		return 2
	}
	switch args[0] {
	case "install":
		return a.install(ctx)
	case "doctor":
		fl := flag.NewFlagSet("doctor", flag.ContinueOnError)
		fl.SetOutput(a.out)
		check := fl.Bool("check", false, "report problems without repairing them")
		if err := fl.Parse(args[1:]); err != nil {
			return 2
		}
		return a.doctor(ctx, !*check)
	case "update":
		return a.update(ctx)
	case "help", "-h", "--help":
		usage(a.out)
		return 0
	default:
		_, _ = fmt.Fprintf(a.out, "devtools: unknown command %q\n", args[0])
		usage(a.out)
		return 2
	}
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `usage: devtools <command>

  install          install every development tool at its latest version, set git hooks
  doctor [-check]  diagnose the setup and repair what it can (-check only reports)
  update           move go.mod to the latest Go toolchain and upgrade every tool and Flutter
`)
}

// findRoot walks up from dir to the directory that holds go.mod.
func findRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found; run from inside the repository")
		}
		dir = parent
	}
}
