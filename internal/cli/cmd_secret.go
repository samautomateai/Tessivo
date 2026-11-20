package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/keychain"
)

func cmdSecret(ctx context.Context, args []string, env Env) error {
	if len(args) != 2 {
		return usagef("secret: want set|delete|check <ref>")
	}
	sub, ref := args[0], args[1]
	if err := keychain.ValidRef(ref); err != nil {
		return usageError{err.Error()}
	}
	switch sub {
	case "set":
		v, err := readSecret(ctx, env, "Value for "+ref+": ")
		if err != nil {
			return err
		}
		if err := env.Keychain.Set(ref, v); err != nil {
			return err
		}
		_, err = fmt.Fprintf(env.Stdout, "stored %s in the OS keychain\n", ref)
		return wrapWrite(err)
	case "delete":
		if err := env.Keychain.Delete(ref); err != nil {
			return err
		}
		_, err := fmt.Fprintf(env.Stdout, "deleted %s\n", ref)
		return wrapWrite(err)
	case "check":
		if _, err := env.Secrets.Get(ref); err != nil {
			return fmt.Errorf("%s is not set (environment or keychain): %w", ref, err)
		}
		_, err := fmt.Fprintf(env.Stdout, "%s is set\n", ref) // never print the value
		return wrapWrite(err)
	default:
		return usagef("secret: unknown subcommand %q", sub)
	}
}

// readSecret reads one value from stdin. On a terminal it prompts on stderr
// and turns echo off (Unix), so the value is not shown or stored in history.
func readSecret(ctx context.Context, env Env, prompt string) (string, error) {
	if env.StdinIsTTY {
		_, _ = fmt.Fprint(env.Stderr, prompt)
		restore := noEcho(ctx, env)
		defer func() {
			restore()
			_, _ = fmt.Fprintln(env.Stderr)
		}()
	}
	r := bufio.NewReader(io.LimitReader(env.Stdin, keychain.MaxSecretBytes+2))
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read secret: %w", err)
	}
	v := strings.TrimRight(line, "\r\n")
	if v == "" {
		return "", usagef("secret: empty value")
	}
	return v, nil
}

// noEcho disables terminal echo with stty on Unix; it returns a restore func.
func noEcho(ctx context.Context, env Env) func() {
	f, ok := env.Stdin.(*os.File)
	if !ok || runtime.GOOS == "windows" {
		return func() {}
	}
	stty := func(arg string) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "stty", arg)
		cmd.Stdin = f
		_ = cmd.Run()
	}
	stty("-echo")
	return func() { stty("echo") }
}
