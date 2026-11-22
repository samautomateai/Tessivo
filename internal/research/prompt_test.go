package research

import (
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/llm"
)

func items() []Item {
	return []Item{
		{N: 1, Kind: ItemSource, ID: "s1", Label: "minutes.md", Level: LevelFull, Text: "Ship in March."},
		{
			N: 2, Kind: ItemSource, ID: "s2", Label: "payments-service", Level: LevelFull,
			Files: []File{{Path: "auth/session.go", Text: "package auth"}},
		},
		{N: 3, Kind: ItemNote, ID: "n1", Label: "My note", Text: "Remember the audit."},
	}
}

func TestBuildFencesAndNumbers(t *testing.T) {
	b := Build("Board", "Board papers for Q3", items(), nil, "When do we ship?", 100000)
	sys, user := b.Messages[0].Content, b.Messages[len(b.Messages)-1].Content
	if b.Messages[0].Role != llm.RoleSystem || !strings.Contains(sys, "[n]") ||
		!strings.Contains(sys, "Board papers for Q3") {
		t.Fatalf("system: %q", sys)
	}
	for _, want := range []string{
		`<source n="1"`, `level="full"`, "Ship in March.", `<file path="auth/session.go">`,
		`<note n="3"`, "Remember the audit.", "When do we ship?",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if len(b.Items) != 3 || len(b.Cut) != 0 || b.Tokens == 0 {
		t.Fatalf("built: %+v", b)
	}
}

func TestBuildCutsLargestFirst(t *testing.T) {
	its := []Item{
		{N: 1, Kind: ItemSource, ID: "big", Label: "big.txt", Level: LevelFull, Text: strings.Repeat("word ", 20000)},
		{N: 2, Kind: ItemSource, ID: "small", Label: "small.txt", Level: LevelFull, Text: "tiny text"},
	}
	b := Build("N", "", its, nil, "q", 4000)
	user := b.Messages[len(b.Messages)-1].Content
	if !strings.Contains(user, "tiny text") || !strings.Contains(user, "[… truncated]") {
		t.Fatalf("small source must stay whole and big one cut:\n%.300s", user)
	}
	if len(b.Cut) != 1 || b.Cut[0] != "big.txt" || b.Tokens < 20000 {
		t.Fatalf("cut %v tokens %d", b.Cut, b.Tokens)
	}
}

func TestBuildHistoryTrimmed(t *testing.T) {
	var hist []chat.Message
	for i := range 40 {
		role := chat.RoleUser
		if i%2 == 1 {
			role = chat.RoleAssistant
		}
		hist = append(hist, chat.Message{Role: role, Content: strings.Repeat("h ", 400)})
	}
	b := Build("N", "", nil, hist, "latest?", 3000)
	if len(b.Messages) >= 42 || b.Messages[1].Role != llm.RoleUser {
		t.Fatalf("history not trimmed or starts on an assistant turn: %d", len(b.Messages))
	}
	if b.Messages[len(b.Messages)-1].Content != "latest?" {
		t.Fatalf("question not last")
	}
}
