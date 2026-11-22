package notebook

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptChecksAndDelete(t *testing.T) {
	nb, err := Create(filepath.Join(t.TempDir(), "nb"), "", "analyze")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []Prompt{
		{Name: "ok", Text: ""},
		{Name: "ok", Text: "   "},
		{Name: "ok", Text: strings.Repeat("x", MaxPromptText+1)},
		{Name: "../x", Text: "t"},
	} {
		if err := nb.SavePrompt(bad); err == nil {
			t.Errorf("saved %q (%d chars)", bad.Name, len(bad.Text))
		}
	}
	if err := nb.SavePrompt(Prompt{Name: "risk", Text: "Assess {{change}}"}); err != nil {
		t.Fatal(err)
	}
	if err := nb.DeletePrompt("risk"); err != nil {
		t.Fatal(err)
	}
	if names, _ := nb.ListPrompts(); len(names) != 0 {
		t.Fatalf("still listed: %v", names)
	}
	if err := nb.DeletePrompt("risk"); err == nil {
		t.Fatal("deleting a missing prompt succeeded")
	}
	if err := nb.DeletePrompt("../x"); err == nil {
		t.Fatal("bad name accepted")
	}
}
