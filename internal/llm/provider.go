package llm

import (
	"context"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
)

// ProviderClient is a client for one configured provider.
type ProviderClient interface {
	Chatter
	Models(ctx context.Context) ([]Model, error)
}

// NewProviderClient returns the client for p's kind: OpenAI-compatible
// (default), Anthropic or Gemini, with p's headers and timeout.
func NewProviderClient(p config.Provider, key string) ProviderClient {
	timeout := time.Duration(p.TimeoutSeconds) * time.Second
	switch p.Kind {
	case config.KindAnthropic:
		c := NewAnthropic(p.URL(), key, timeout)
		c.Headers = p.Headers
		return c
	case config.KindGemini:
		c := NewGemini(p.URL(), key, timeout)
		c.Headers = p.Headers
		return c
	default:
		c := NewClient(p.URL(), key, timeout)
		c.Headers = p.Headers
		return c
	}
}

// Words the provider clients share.
const (
	choiceAuto   = "auto"
	choiceNone   = "none"
	finishStop   = "stop"
	blockText    = "text"
	blockToolUse = "tool_use"
)
