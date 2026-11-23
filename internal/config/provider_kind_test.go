package config

import "testing"

func TestProviderKinds(t *testing.T) {
	f := Default()
	f.Providers = append(f.Providers,
		Provider{Name: "claude", Kind: KindAnthropic, APIKeyRef: "ANTHROPIC_KEY"},
		Provider{Name: "google", Kind: KindGemini, APIKeyRef: "GEMINI_KEY"})
	if err := f.Validate(); err != nil {
		t.Fatalf("native providers may leave base_url empty: %v", err)
	}
	if got := f.Providers[1].URL(); got != "https://api.anthropic.com" {
		t.Fatalf("anthropic default URL %q", got)
	}
	if got := f.Providers[2].URL(); got != "https://generativelanguage.googleapis.com" {
		t.Fatalf("gemini default URL %q", got)
	}
	f.Providers[1].Kind = "bedrock"
	if err := f.Validate(); err == nil {
		t.Fatal("unknown kind accepted")
	}
	f.Providers[1] = Provider{Name: "plain", APIKeyRef: "K"} // openai-compatible still needs a URL
	if err := f.Validate(); err == nil {
		t.Fatal("openai-compatible provider without base_url accepted")
	}
}
