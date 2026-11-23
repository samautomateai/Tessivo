package llm

import (
	"context"
	"fmt"
	"sync"
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
	c := newKindClient(p, key)
	if p.MaxConcurrent > 0 {
		return withCap(p.Name, p.MaxConcurrent, c)
	}
	return c
}

func newKindClient(p config.Provider, key string) ProviderClient {
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

// slots holds one semaphore per provider name and cap, shared by every
// client of that provider in the process.
var slots sync.Map

type capped struct {
	ProviderClient
	sem chan struct{}
}

func withCap(name string, n int, c ProviderClient) ProviderClient {
	v, _ := slots.LoadOrStore(fmt.Sprintf("%s#%d", name, n), make(chan struct{}, n))
	return &capped{ProviderClient: c, sem: v.(chan struct{})} //nolint:forcetypeassert // only chans are stored
}

func (c *capped) acquire(ctx context.Context) error {
	select {
	case c.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for a free request slot: %w", ctx.Err())
	}
}

// Chat waits for a free slot, then sends.
func (c *capped) Chat(ctx context.Context, req Request) (Response, error) {
	if err := c.acquire(ctx); err != nil {
		return Response{}, err
	}
	defer func() { <-c.sem }()
	return c.ProviderClient.Chat(ctx, req)
}

// ChatStream waits for a free slot, then streams.
func (c *capped) ChatStream(ctx context.Context, req Request, onDelta func(string)) (Response, error) {
	if err := c.acquire(ctx); err != nil {
		return Response{}, err
	}
	defer func() { <-c.sem }()
	return c.ProviderClient.ChatStream(ctx, req, onDelta)
}
