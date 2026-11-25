package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/samautomateai/tessivo/internal/notebook"
)

// promptHistory lists a prompt's versions, newest first, with the start of each text.
func promptHistory(nb *notebook.Notebook, name string, env Env) error {
	vs, err := nb.PromptVersions(name)
	if err != nil {
		return err
	}
	if len(vs) == 0 {
		return fmt.Errorf("prompt %s: not found", name)
	}
	var b strings.Builder
	for i, v := range vs {
		text := strings.ReplaceAll(v.Text, "\n", " ")
		if r := []rune(text); len(r) > 60 {
			text = string(r[:60]) + "…"
		}
		cur := ""
		if i == 0 {
			cur = " (current)"
		}
		fmt.Fprintf(&b, "v%-4d %-20s %s%s\n", v.Version, v.SavedAt, text, cur)
	}
	_, err = fmt.Fprint(env.Stdout, b.String())
	return wrapWrite(err)
}

// promptRestore makes an earlier version current: tess prompt restore <notebook> <name> <version>.
func promptRestore(nb *notebook.Notebook, rest []string, env Env) error {
	if len(rest) != 2 {
		return usagef("prompt restore: want <notebook> <name> <version>")
	}
	v, err := strconv.Atoi(strings.TrimPrefix(rest[1], "v"))
	if err != nil || v < 1 {
		return usagef("prompt restore: version %q is not a number", rest[1])
	}
	if err := nb.RestorePromptVersion(rest[0], v); err != nil {
		return err
	}
	p, err := nb.LoadPrompt(rest[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "restored v%d of %s as v%d\n", v, rest[0], p.Version)
	return wrapWrite(err)
}
