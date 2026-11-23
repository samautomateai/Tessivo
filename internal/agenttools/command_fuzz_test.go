package agenttools

import (
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

// FuzzCommandMatch checks the allow-list matcher never panics and that a
// rule of only "*" arguments matches exactly that many arguments.
func FuzzCommandMatch(f *testing.F) {
	f.Add("tessivo-fuzz-prog", "test *", "test ./...")
	f.Add("tessivo-fuzz-prog", "*", "")
	f.Add("tessivo-fuzz-prog", "", "a b")
	f.Fuzz(func(t *testing.T, program, ruleArgs, callArgs string) {
		rule := config.CommandRule{Program: program, Args: strings.Fields(ruleArgs)}
		args := strings.Fields(callArgs)
		got := matches(rule, program, args)
		if len(args) != len(rule.Args) && got {
			t.Fatalf("matched %d arguments against a rule of %d", len(args), len(rule.Args))
		}
		stars := make([]string, len(args))
		for i := range stars {
			stars[i] = "*"
		}
		if !matches(config.CommandRule{Program: program, Args: stars}, program, args) {
			t.Fatal("a rule of * arguments must match the same number of arguments")
		}
	})
}
