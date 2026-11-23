package llm

import (
	"fmt"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestNewProviderClientPicksTheKind(t *testing.T) {
	for kind, want := range map[string]string{
		"": "*llm.Client", config.KindOpenAI: "*llm.Client",
		config.KindAnthropic: "*llm.Anthropic", config.KindGemini: "*llm.Gemini",
	} {
		c := NewProviderClient(config.Provider{Name: "p", Kind: kind, BaseURL: "https://x.example/v1"}, "k")
		if got := typeName(c); got != want {
			t.Errorf("kind %q: %s, want %s", kind, got, want)
		}
	}
}

func typeName(v any) string { return fmt.Sprintf("%T", v) }
