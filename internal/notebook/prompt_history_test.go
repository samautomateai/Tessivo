package notebook

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestPromptHistory(t *testing.T) {
	nb, err := Create(filepath.Join(t.TempDir(), "ops"), "Ops", "summarize")
	if err != nil {
		t.Fatal(err)
	}
	save := func(text string) Prompt {
		t.Helper()
		if err := nb.SavePrompt(Prompt{Name: "weekly", Text: text}); err != nil {
			t.Fatal(err)
		}
		p, err := nb.LoadPrompt("weekly")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := save("v1"); p.Version != 1 {
		t.Fatalf("first save: version %d", p.Version)
	}
	if p := save("v1"); p.Version != 1 {
		t.Fatalf("the same text must not make a version: %d", p.Version)
	}
	if p := save("v2"); p.Version != 2 {
		t.Fatalf("second text: version %d", p.Version)
	}
	vs, err := nb.PromptVersions("weekly")
	if err != nil || len(vs) != 2 || vs[0].Version != 2 || vs[1].Text != "v1" {
		t.Fatalf("%v %+v", err, vs)
	}
	if err := nb.RestorePromptVersion("weekly", 1); err != nil {
		t.Fatal(err)
	}
	p, _ := nb.LoadPrompt("weekly")
	if p.Version != 3 || p.Text != "v1" {
		t.Fatalf("restore is a new save: %+v", p)
	}
	if vs, _ := nb.PromptVersions("weekly"); len(vs) != 3 || vs[1].Text != "v2" {
		t.Fatalf("the restored-over version stays in history: %+v", vs)
	}
	if err := nb.RestorePromptVersion("weekly", 99); err == nil {
		t.Fatal("restoring a missing version must fail")
	}
	if names, _ := nb.ListPrompts(); len(names) != 1 {
		t.Fatalf("history must not show as prompts: %v", names)
	}
}

func TestPromptHistoryKeepsFifty(t *testing.T) {
	nb, _ := Create(filepath.Join(t.TempDir(), "ops"), "Ops", "summarize")
	for i := range MaxPromptVersions + 5 {
		if err := nb.SavePrompt(Prompt{Name: "p", Text: fmt.Sprintf("text %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	vs, _ := nb.PromptVersions("p")
	if len(vs) != MaxPromptVersions || vs[0].Version != MaxPromptVersions+5 {
		t.Fatalf("%d versions, newest %d", len(vs), vs[0].Version)
	}
	if err := nb.DeletePrompt("p"); err != nil {
		t.Fatal(err)
	}
	if vs, _ := nb.PromptVersions("p"); len(vs) != 0 {
		t.Fatal("deleting a prompt drops its history")
	}
}
