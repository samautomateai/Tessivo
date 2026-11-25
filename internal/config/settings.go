// Package config holds Tessivo's application configuration and the layered
// run settings that resolve app → notebook → work type → prompt → run.
package config

import "fmt"

// Thinking effort levels accepted by RunSettings.ThinkingEffort.
const (
	ThinkingOff    = "off"
	ThinkingLow    = "low"
	ThinkingMedium = "medium"
	ThinkingHigh   = "high"
)

// Execution modes accepted by RunSettings.Mode.
const (
	ModePipeline = "pipeline"
	ModeAgent    = "agent"

	// ChainPerSource condenses each source on its own first, then writes the output from the notes.
	ChainPerSource = "per_source"
)

// RunSettings are the knobs that can be set at any layer. Nil or empty
// fields mean "not set at this layer" and fall through to the layer below.
type RunSettings struct {
	Model           string   `json:"model,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`
	ContextBudget   *int     `json:"context_budget,omitempty"`
	ThinkingEffort  string   `json:"thinking_effort,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	// Chain is "" (one call; condense only when sources do not fit) or ChainPerSource.
	Chain string `json:"chain,omitempty"`
}

// Validate reports invalid values in a single layer.
func (s RunSettings) Validate() error {
	switch s.ThinkingEffort {
	case "", ThinkingOff, ThinkingLow, ThinkingMedium, ThinkingHigh:
	default:
		return fmt.Errorf("thinking_effort %q: want off, low, medium or high", s.ThinkingEffort)
	}
	switch s.Mode {
	case "", ModePipeline, ModeAgent:
	default:
		return fmt.Errorf("mode %q: want pipeline or agent", s.Mode)
	}
	if s.Chain != "" && s.Chain != ChainPerSource {
		return fmt.Errorf("chain %q: want per_source or nothing", s.Chain)
	}
	if s.Temperature != nil && (*s.Temperature < 0 || *s.Temperature > 2) {
		return fmt.Errorf("temperature %v: want 0 to 2", *s.Temperature)
	}
	if s.MaxOutputTokens != nil && *s.MaxOutputTokens <= 0 {
		return fmt.Errorf("max_output_tokens %d: must be positive", *s.MaxOutputTokens)
	}
	if s.ContextBudget != nil && *s.ContextBudget <= 0 {
		return fmt.Errorf("context_budget %d: must be positive", *s.ContextBudget)
	}
	return nil
}

// Layer is one named level of settings, e.g. "app", "notebook", "run".
type Layer struct {
	Name     string
	Settings RunSettings
}

// Effective is the resolved settings plus, per field, the layer it came from.
type Effective struct {
	RunSettings
	Sources map[string]string
}

// Resolve merges layers in order; later layers override earlier ones.
// Pass layers from least to most specific: app, notebook, work type, prompt, run.
func Resolve(layers ...Layer) Effective {
	e := Effective{Sources: map[string]string{}}
	for _, l := range layers {
		s := l.Settings
		if s.Model != "" {
			e.Model, e.Sources["model"] = s.Model, l.Name
		}
		if s.Temperature != nil {
			e.Temperature, e.Sources["temperature"] = s.Temperature, l.Name
		}
		if s.MaxOutputTokens != nil {
			e.MaxOutputTokens, e.Sources["max_output_tokens"] = s.MaxOutputTokens, l.Name
		}
		if s.ContextBudget != nil {
			e.ContextBudget, e.Sources["context_budget"] = s.ContextBudget, l.Name
		}
		if s.ThinkingEffort != "" {
			e.ThinkingEffort, e.Sources["thinking_effort"] = s.ThinkingEffort, l.Name
		}
		if s.Chain != "" {
			e.Chain, e.Sources["chain"] = s.Chain, l.Name
		}
		if s.Mode != "" {
			e.Mode, e.Sources["mode"] = s.Mode, l.Name
		}
	}
	return e
}

// Ptr returns a pointer to v; handy for optional settings.
func Ptr[T any](v T) *T { return &v }
