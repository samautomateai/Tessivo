package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// cmdPrompt manages a notebook's saved prompts.
func cmdPrompt(args []string, env Env) error {
	if len(args) < 2 {
		return usagef("prompt: want list|show|save|rm <notebook> [name]")
	}
	sub, dir, rest := args[0], args[1], args[2:]
	switch sub {
	case subList, subShow, subSave, "rm":
	default:
		return usagef("prompt: unknown subcommand %q", sub)
	}
	if sub != subList && len(rest) == 0 {
		return usagef("prompt %s: want <notebook> <name>", sub)
	}
	nb, err := notebook.Open(dir)
	if err != nil {
		return err
	}
	switch sub {
	case subList:
		return promptList(nb, env)
	case subShow:
		return promptShow(nb, rest[0], env)
	case subSave:
		return promptSave(nb, rest, env)
	default:
		return nb.DeletePrompt(rest[0])
	}
}

func promptList(nb *notebook.Notebook, env Env) error {
	names, err := nb.ListPrompts()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "NAME\tMODEL\tTEXT")
	for _, n := range names {
		p, err := nb.LoadPrompt(n)
		if err != nil {
			continue
		}
		first, _, _ := strings.Cut(p.Text, "\n")
		if r := []rune(first); len(r) > 60 {
			first = string(r[:60]) + "…"
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", p.Name, p.Settings.Model, first)
	}
	return wrapWrite(tw.Flush())
}

func promptShow(nb *notebook.Notebook, name string, env Env) error {
	p, err := nb.LoadPrompt(name)
	if err != nil {
		return err
	}
	var sets []string
	if p.Settings.Model != "" {
		sets = append(sets, "model "+p.Settings.Model)
	}
	if t := p.Settings.Temperature; t != nil {
		sets = append(sets, fmt.Sprintf("temperature %g", *t))
	}
	if m := p.Settings.MaxOutputTokens; m != nil {
		sets = append(sets, fmt.Sprintf("max tokens %d", *m))
	}
	if len(sets) > 0 {
		_, _ = fmt.Fprintf(env.Stdout, "# %s\n", strings.Join(sets, ", "))
	}
	_, err = fmt.Fprintln(env.Stdout, p.Text)
	return wrapWrite(err)
}

// promptSave saves rest[0] with the remaining words, or stdin, as its text.
func promptSave(nb *notebook.Notebook, rest []string, env Env) error {
	fs := newFlags("prompt save", env)
	model := fs.String("model", "", "model alias for this prompt")
	temp := fs.Float64("temperature", -1, "temperature for this prompt")
	maxTok := fs.Int("max-tokens", 0, "max output tokens for this prompt")
	pos, err := parse(fs, rest)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		return usagef("prompt save: want <notebook> <name> [text]")
	}
	text := strings.Join(pos[1:], " ")
	if text == "" && !env.StdinIsTTY && env.Stdin != nil {
		b, err := io.ReadAll(io.LimitReader(env.Stdin, notebook.MaxPromptText+1))
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		text = string(b)
	}
	if strings.TrimSpace(text) == "" {
		return usagef("prompt save: give the text as arguments or on stdin")
	}
	p := notebook.Prompt{Name: pos[0], Text: strings.TrimRight(text, "\n")}
	p.Settings.Model = *model
	if *temp >= 0 {
		p.Settings.Temperature = temp
	}
	if *maxTok > 0 {
		p.Settings.MaxOutputTokens = maxTok
	}
	if err := nb.SavePrompt(p); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "saved prompt %s\n", p.Name)
	return wrapWrite(err)
}
