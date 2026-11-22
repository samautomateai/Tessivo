package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/samautomateai/tessivo/internal/config"
)

// configKeys handles tess config get|set|export|import; p is the config path.
func configKeys(args []string, p string, env Env) error {
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return usagef("config get: want <key>, e.g. defaults.model")
		}
		f, err := loadConfig(env)
		if err != nil {
			return err
		}
		v, err := f.Get(args[1])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(env.Stdout, v)
		return wrapWrite(err)
	case subSet:
		if len(args) != 3 {
			return usagef("config set: want <key> <value>, e.g. limits.max_tokens_per_day 50000")
		}
		f, err := loadConfig(env)
		if err != nil {
			return err
		}
		if f, err = f.Set(args[1], args[2]); err != nil {
			return err
		}
		return f.Save(p)
	case "export":
		return configExport(args[1:], env)
	default:
		return configImport(args[1:], p, env)
	}
}

func configExport(args []string, env Env) error {
	if len(args) > 1 {
		return usagef("config export: want [file]")
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	b, err := f.Marshal()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		_, err = env.Stdout.Write(b)
		return wrapWrite(err)
	}
	if err := os.WriteFile(args[0], b, 0o600); err != nil {
		return fmt.Errorf("export config: %w", err)
	}
	return nil
}

// configImport replaces the config with a validated file, keeping the old one as <config>.bak.
func configImport(args []string, p string, env Env) error {
	fset := newFlags("config import", env)
	force := fset.Bool("force", false, "replace the existing config (it is kept as .bak)")
	pos, err := parse(fset, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("config import: want <file> [--force]")
	}
	in, err := os.Open(pos[0])
	if err != nil {
		return fmt.Errorf("import config: %w", err)
	}
	defer func() { _ = in.Close() }()
	b, err := io.ReadAll(io.LimitReader(in, config.MaxConfigBytes+1))
	if err != nil {
		return fmt.Errorf("import config: %w", err)
	}
	f, err := config.Parse(b)
	if err != nil {
		return err
	}
	old, err := os.ReadFile(p) //nolint:gosec // G304: the user's own config path
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return fmt.Errorf("read config: %w", err)
	case !*force:
		return fmt.Errorf("a config exists at %s (use --force to replace it; it is kept as .bak)", p)
	default:
		if err := os.WriteFile(p+".bak", old, 0o600); err != nil { //nolint:gosec // G703: next to the user's own config
			return fmt.Errorf("back up config: %w", err)
		}
	}
	if err := f.Save(p); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "imported %s into %s\n", pos[0], p)
	return wrapWrite(err)
}
