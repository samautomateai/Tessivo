package config

import (
	"fmt"
	"strings"
)

// CommandRule is one entry of a notebook's command allow-list for agent mode:
// a program and its arguments, where an argument "*" stands for exactly one
// free argument. {"go", ["test", "*"]} allows `go test ./...` but not
// `go test -exec x ./...`. Calls that match run without asking.
type CommandRule struct {
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
}

// Limits on command allow-lists.
const (
	MaxCommandRules = 100
	MaxCommandArgs  = 64
	MaxCommandArg   = 4096
)

// CheckCommandRules validates an allow-list.
func CheckCommandRules(rules []CommandRule) error {
	if len(rules) > MaxCommandRules {
		return fmt.Errorf("commands: %d entries; at most %d", len(rules), MaxCommandRules)
	}
	for i, r := range rules {
		if strings.TrimSpace(r.Program) == "" || len(r.Args) > MaxCommandArgs {
			return fmt.Errorf("commands[%d]: want a program and at most %d arguments", i, MaxCommandArgs)
		}
		for _, a := range append([]string{r.Program}, r.Args...) {
			if len(a) > MaxCommandArg || strings.ContainsRune(a, 0) {
				return fmt.Errorf("commands[%d]: an argument is over %d bytes or has a NUL byte", i, MaxCommandArg)
			}
		}
	}
	return nil
}
