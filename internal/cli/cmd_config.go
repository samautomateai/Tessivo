package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/worktype"
)

func configPath(env Env) (string, error) {
	if env.ConfigPath != "" {
		return env.ConfigPath, nil
	}
	return config.DefaultPath()
}

func loadConfig(env Env) (config.File, error) {
	p, err := configPath(env)
	if err != nil {
		return config.File{}, err
	}
	f, err := config.Load(p)
	if errors.Is(err, os.ErrNotExist) {
		return config.File{}, fmt.Errorf("no config at %s: run 'tess config init' first", p)
	}
	return f, err
}

func cmdConfig(args []string, env Env) error {
	if len(args) == 0 {
		return usagef("config: want init, show, path, get, set, export or import")
	}
	p, err := configPath(env)
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		_, err := fmt.Fprintln(env.Stdout, p)
		return wrapWrite(err)
	case "init":
		fs := newFlags("config init", env)
		force := fs.Bool("force", false, "overwrite an existing config")
		if _, err := parse(fs, args[1:]); err != nil {
			return err
		}
		if _, err := os.Stat(p); err == nil && !*force {
			return fmt.Errorf("config already exists at %s (use --force to overwrite)", p)
		}
		if err := config.Default().Save(p); err != nil {
			return err
		}
		_, err := fmt.Fprintf(env.Stdout, "wrote %s\nstore your key: tess secret set TESSIVO_LITELLM_KEY\n", p)
		return wrapWrite(err)
	case "show":
		f, err := loadConfig(env)
		if err != nil {
			return err
		}
		b, _ := json.MarshalIndent(f, "", "  ")
		_, err = fmt.Fprintln(env.Stdout, string(b))
		return wrapWrite(err)
	case "get", subSet, subExport, "import":
		return configKeys(args, p, env)
	default:
		return usagef("config: unknown subcommand %q", args[0])
	}
}

func listModels(ctx context.Context, p config.Provider, sec secrets.Resolver) ([]llm.Model, error) {
	key := ""
	if p.APIKeyRef != "" {
		v, err := sec.Get(p.APIKeyRef)
		if err != nil {
			return nil, fmt.Errorf("provider %q api key: %w", p.Name, err)
		}
		key = v
	}
	c := llm.NewClient(p.BaseURL, key, time.Duration(p.TimeoutSeconds)*time.Second)
	c.Headers = p.Headers
	return c.Models(ctx)
}

func cmdModels(ctx context.Context, args []string, env Env) error {
	fs := newFlags("models", env)
	only := fs.String("provider", "", "only this provider")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PROVIDER\tMODEL")
	for _, p := range f.Providers {
		if *only != "" && p.Name != *only {
			continue
		}
		ms, err := env.ListModels(ctx, p, env.Secrets)
		if err != nil {
			return fmt.Errorf("provider %s: %w", p.Name, err)
		}
		for _, m := range ms {
			_, _ = fmt.Fprintf(tw, "%s\t%s\n", p.Name, m.ID)
		}
	}
	return wrapWrite(tw.Flush())
}

func cmdWorkTypes(env Env) error {
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tQUESTION\tSUGGESTED CONTEXT")
	for _, w := range worktype.All() {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", w.ID, w.Question, strings.Join(w.SuggestedContext, ", "))
	}
	return wrapWrite(tw.Flush())
}
