package service

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

const revised = `{"title":"Board digest v2","tldr":"t","key_points":["k","cited"],"per_source":[],"action_items":[]}`

func TestNewVersionSavesLinkedOutput(t *testing.T) {
	f := newThreadFixture(t, step{text: "Sure, I will cite files."}, step{text: revised})
	r, _, err := f.send(t, "please cite files")
	if err != nil {
		t.Fatal(err)
	}
	parentFile := filepath.Join(f.nb.Dir, notebook.OutputsDir, f.output+".json")
	before, _ := os.ReadFile(parentFile)
	out, err := f.s.NewVersion(context.Background(), f.nbID, f.output, "")
	if err != nil {
		t.Fatal(err)
	}
	reqs := f.model.requests()
	user := reqs[len(reqs)-1].Messages[1].Content
	if !strings.Contains(user, "Board digest") || !strings.Contains(user, "please cite files") {
		t.Fatalf("revision request lacks the parent or the conversation:\n%s", user)
	}
	info, err := f.s.GetOutput(f.nbID, out.OutputID)
	if err != nil {
		t.Fatal(err)
	}
	if info.ParentID != f.output || info.FromThread != r.Assistant.ID || info.Version != 2 {
		t.Fatalf("lineage %+v", info.Output)
	}
	if after, _ := os.ReadFile(parentFile); !bytes.Equal(before, after) {
		t.Fatal("parent output changed")
	}
	sess, _ := f.s.GetThread(f.nbID, f.output)
	last := sess.Messages[len(sess.Messages)-1]
	if last.Role != chat.RoleNote || !strings.Contains(last.Content, "v2") {
		t.Fatalf("no note in the parent thread: %+v", last)
	}
	list, _ := f.s.ListOutputs(f.nbID, "")
	got := map[string]int{}
	for _, o := range list {
		got[o.ID] = o.Version
	}
	if len(list) != 2 || got[out.OutputID] != 2 || got[f.output] != 1 {
		t.Fatalf("list versions: %v", got)
	}
}

func TestNewVersionInvalidSavesNothing(t *testing.T) {
	f := newThreadFixture(t, step{text: "ok"}, step{text: "nope"}, step{text: "nope"}, step{text: "nope"})
	if _, _, err := f.send(t, "change it"); err != nil {
		t.Fatal(err)
	}
	_, err := f.s.NewVersion(context.Background(), f.nbID, f.output, "")
	if !errors.Is(err, llm.ErrInvalidOutput) {
		t.Fatalf("want invalid output, got %v", err)
	}
	if list, _ := f.s.ListOutputs(f.nbID, ""); len(list) != 1 {
		t.Fatalf("outputs: %d", len(list))
	}
}

func TestNewVersionAgentNotebookUsesPipeline(t *testing.T) {
	f := newThreadFixture(t, step{text: "ok"}, step{text: revised})
	nb := f.reopen(t)
	nb.Manifest.Settings.Mode = config.ModeAgent
	if err := nb.Save(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.send(t, "change it"); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.NewVersion(context.Background(), f.nbID, f.output, "")
	if err != nil || len(out.Paused) != 0 || out.OutputID == "" {
		t.Fatalf("%v %+v", err, out)
	}
	reqs := f.model.requests()
	if last := reqs[len(reqs)-1]; len(last.Tools) != 0 || last.Format == nil {
		t.Fatalf("want a structured pipeline call: tools %d", len(last.Tools))
	}
}

func TestNewVersionFromMessage(t *testing.T) {
	f := newThreadFixture(t, step{text: "first answer"}, step{text: "second answer"}, step{text: revised})
	first, _, _ := f.send(t, "first ask")
	if _, _, err := f.send(t, "second ask"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, first.Assistant.ID); err != nil {
		t.Fatal(err)
	}
	reqs := f.model.requests()
	user := reqs[len(reqs)-1].Messages[1].Content
	if !strings.Contains(user, "first ask") || strings.Contains(user, "second ask") {
		t.Fatalf("requests not cut at the message:\n%s", user)
	}
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, "nope"); !IsInvalid(err) {
		t.Fatalf("unknown message: %v", err)
	}
}

func TestNewVersionEmptyThread(t *testing.T) {
	f := newThreadFixture(t)
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, ""); !IsInvalid(err) {
		t.Fatalf("empty thread: %v", err)
	}
}

func TestNewVersionMissingRun(t *testing.T) {
	f := newThreadFixture(t)
	ref, _ := f.s.loadOutput(f.nbID, f.output)
	if err := os.Remove(filepath.Join(f.nb.Dir, notebook.RunsDir, ref.out.RunID+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing run: %v", err)
	}
}

func TestNewVersionBusyThread(t *testing.T) {
	f := newThreadFixture(t)
	st, _ := f.s.threadStore(f.nb)
	release, err := st.Acquire(f.output)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := f.s.NewVersion(context.Background(), f.nbID, f.output, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("busy: %v", err)
	}
}

func TestVersionChainCycle(t *testing.T) {
	a, b := notebook.NewID(), notebook.NewID()
	outs := []notebook.Output{{ID: a, ParentID: b}, {ID: b, ParentID: a}, {ID: "c", ParentID: "gone"}}
	v := Versions(outs)
	if v[a] < 1 || v[a] > 3 || v[b] < 1 || v[b] > 3 || v["c"] != 2 {
		t.Fatalf("versions %v", v)
	}
}
