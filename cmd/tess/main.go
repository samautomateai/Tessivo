// Command tess is the Tessivo CLI.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/samautomateai/tessivo/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	info, _ := os.Stdin.Stat()
	tty := info == nil || info.Mode()&os.ModeCharDevice != 0
	code := cli.Main(ctx, os.Args[1:], cli.Env{
		Stdin: os.Stdin, StdinIsTTY: tty, Stdout: os.Stdout, Stderr: os.Stderr,
	})
	stop()
	os.Exit(code)
}
