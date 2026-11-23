package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// notebookCommands manages the run_command allow-list of agent mode:
//
//	tess notebook commands <notebook>                     list
//	tess notebook commands <notebook> add <program> [arg|*]...
//	tess notebook commands <notebook> remove <number>
func notebookCommands(args []string, env Env) error {
	if len(args) == 0 {
		return usagef("notebook commands: want <notebook> [add <program> [args]... | remove <number>]")
	}
	nb, err := notebook.Open(args[0])
	if err != nil {
		return err
	}
	rest := args[1:]
	switch {
	case len(rest) == 0:
		for i, r := range nb.Manifest.Agent.Commands {
			line := strings.Join(append([]string{r.Program}, r.Args...), " ")
			if _, err := fmt.Fprintf(env.Stdout, "%d. %s\n", i+1, line); err != nil {
				return wrapWrite(err)
			}
		}
		return nil
	case rest[0] == subAdd && len(rest) >= 2:
		rule := config.CommandRule{Program: rest[1], Args: rest[2:]}
		return nb.Update(func(m *notebook.Manifest) error {
			next := append(append([]config.CommandRule{}, m.Agent.Commands...), rule)
			if err := config.CheckCommandRules(next); err != nil {
				return usageError{err.Error()}
			}
			m.Agent.Commands = next
			return nil
		})
	case rest[0] == subRemove && len(rest) == 2:
		n, err := strconv.Atoi(rest[1])
		cmds := nb.Manifest.Agent.Commands
		if err != nil || n < 1 || n > len(cmds) {
			return usagef("notebook commands remove: want a number from the list")
		}
		nb.Manifest.Agent.Commands = append(append([]config.CommandRule{}, cmds[:n-1]...), cmds[n:]...)
		return nb.Save() // refused if the list changed since it was read: numbers must match
	default:
		return usagef("notebook commands: want <notebook> [add <program> [args]... | remove <number>]")
	}
}
