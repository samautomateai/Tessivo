package service

import (
	"testing"

	"github.com/samautomateai/tessivo/internal/llm"
)

func FuzzParseSuggestionCall(f *testing.F) {
	f.Add(toolSuggestMemory, `{"text":"cite file paths","reason":"r"}`)
	f.Add(toolSuggestPrompt, `{"text":"Assess {{pr}}"}`)
	f.Add("run_shell", `{"text":"x"}`)
	f.Add(toolSuggestMemory, `[1,2]`)
	f.Add(toolSuggestMemory, `{"text":"\u0000￿"}`)
	f.Fuzz(func(t *testing.T, name, args string) {
		sg, err := parseSuggestionCall(llm.ToolCall{ID: "c", Name: name, Arguments: args})
		if err != nil {
			return
		}
		if err := sg.Check(MaxMemoryItem); err != nil {
			t.Fatalf("accepted a suggestion that fails Check: %v", err)
		}
		if name != toolSuggestMemory && name != toolSuggestPrompt {
			t.Fatalf("accepted unknown tool %q", name)
		}
	})
}
