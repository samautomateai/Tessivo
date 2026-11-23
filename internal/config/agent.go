package config

import (
	"errors"
	"fmt"
	"sort"
)

// Tool rules: what happens when the agent calls a tool.
const (
	RuleAllow = "allow"
	RuleAsk   = "ask"
	RuleBlock = "block"
)

// Agent defaults.
const (
	DefaultAgentMaxSteps       = 40
	DefaultAgentMaxTokens      = 400000
	DefaultAgentMaxMinutes     = 30
	DefaultAgentMaxToolCalls   = 200
	DefaultApprovalExpiryDays  = 7
	maxToolRules               = 500
	maxToolRuleNameLen         = 200
	maxAgentLimitSteps         = 1000
	maxAgentLimitMinutes       = 24 * 60
	maxAgentLimitToolCalls     = 10000
	maxApprovalExpiryDaysLimit = 365
)

// AgentConfig is the app-level agent settings: default tool rules and limits.
type AgentConfig struct {
	Tools              map[string]string `json:"tools,omitempty"`
	MaxSteps           int               `json:"max_steps,omitempty"`
	MaxTokens          int               `json:"max_tokens,omitempty"`
	MaxMinutes         int               `json:"max_minutes,omitempty"`
	MaxToolCalls       int               `json:"max_tool_calls,omitempty"`
	ApprovalExpiryDays int               `json:"approval_expiry_days,omitempty"`
}

// WithDefaults fills unset limits with the defaults.
func (c AgentConfig) WithDefaults() AgentConfig {
	if c.MaxSteps == 0 {
		c.MaxSteps = DefaultAgentMaxSteps
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = DefaultAgentMaxTokens
	}
	if c.MaxMinutes == 0 {
		c.MaxMinutes = DefaultAgentMaxMinutes
	}
	if c.MaxToolCalls == 0 {
		c.MaxToolCalls = DefaultAgentMaxToolCalls
	}
	if c.ApprovalExpiryDays == 0 {
		c.ApprovalExpiryDays = DefaultApprovalExpiryDays
	}
	return c
}

// Validate checks the rules and that limits are in range.
func (c AgentConfig) Validate() error {
	errs := []error{CheckToolRules("agent.tools", c.Tools)}
	check := func(name string, v, hi int) {
		if v < 0 || v > hi {
			errs = append(errs, fmt.Errorf("agent.%s: %d is out of range 0–%d", name, v, hi))
		}
	}
	check("max_steps", c.MaxSteps, maxAgentLimitSteps)
	check("max_tokens", c.MaxTokens, 100_000_000)
	check("max_minutes", c.MaxMinutes, maxAgentLimitMinutes)
	check("max_tool_calls", c.MaxToolCalls, maxAgentLimitToolCalls)
	check("approval_expiry_days", c.ApprovalExpiryDays, maxApprovalExpiryDaysLimit)
	return errors.Join(errs...)
}

// CheckToolRules validates a tool → rule map. layer names it in errors.
func CheckToolRules(layer string, rules map[string]string) error {
	if len(rules) > maxToolRules {
		return fmt.Errorf("%s: %d rules; at most %d", layer, len(rules), maxToolRules)
	}
	names := make([]string, 0, len(rules))
	for n := range rules {
		names = append(names, n)
	}
	sort.Strings(names)
	var errs []error
	for _, n := range names {
		switch {
		case n == "" || len(n) > maxToolRuleNameLen:
			errs = append(errs, fmt.Errorf("%s: tool name must be 1–%d characters", layer, maxToolRuleNameLen))
		case !ValidRule(rules[n]):
			errs = append(errs, fmt.Errorf("%s: tool %q: rule %q: want allow, ask or block", layer, n, rules[n]))
		}
	}
	return errors.Join(errs...)
}

// ValidRule reports whether s is allow, ask or block.
func ValidRule(s string) bool {
	return s == RuleAllow || s == RuleAsk || s == RuleBlock
}
