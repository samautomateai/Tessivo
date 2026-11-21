// Package agent is Tessivo's agent mode: a tool-using loop over the LLM
// router that pauses durably when a tool needs the user's approval, and ends
// with the same validated structured output as the pipeline.
package agent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
)

// Decision is what happens when the model calls a tool.
type Decision string

// Decisions.
const (
	Allow Decision = config.RuleAllow
	Ask   Decision = config.RuleAsk
	Block Decision = config.RuleBlock
)

// builtinRules apply when no layer has a rule: read-only tools run, anything
// else asks.
var builtinRules = map[string]string{
	"read_file": config.RuleAllow, "list_directory": config.RuleAllow, "glob": config.RuleAllow,
	"grep": config.RuleAllow, finishTool: config.RuleAllow,
}

// ParseDecision converts allow, ask or block.
func ParseDecision(s string) (Decision, error) {
	if !config.ValidRule(s) {
		return "", fmt.Errorf("rule %q: want allow, ask or block", s)
	}
	return Decision(s), nil
}

// Policy holds the rule layers, most specific first: this run, the notebook,
// the app. Built-in defaults apply last.
type Policy struct {
	Run, Notebook, App map[string]string
}

// Decide returns the rule for tool from the most specific layer that has one.
// Within a layer an exact name beats the longest matching "prefix*" pattern.
func (p Policy) Decide(tool string) Decision {
	for _, layer := range []map[string]string{p.Run, p.Notebook, p.App, builtinRules} {
		if r, ok := lookup(layer, tool); ok {
			return Decision(r)
		}
	}
	return Ask
}

func lookup(rules map[string]string, tool string) (string, bool) {
	if r, ok := rules[tool]; ok {
		return r, true
	}
	best, bestLen := "", -1
	for pat, r := range rules {
		prefix, isPat := strings.CutSuffix(pat, "*")
		if isPat && strings.HasPrefix(tool, prefix) && len(prefix) > bestLen {
			best, bestLen = r, len(prefix)
		}
	}
	return best, bestLen >= 0
}

// Check validates every layer.
func (p Policy) Check() error {
	return errors.Join(
		config.CheckToolRules("run", p.Run),
		config.CheckToolRules("notebook", p.Notebook),
		config.CheckToolRules("app", p.App),
	)
}

// finishTool lets the model end the loop early.
const finishTool = "finish"
