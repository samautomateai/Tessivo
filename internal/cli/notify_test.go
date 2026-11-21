package cli

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/notify"
	"github.com/samautomateai/tessivo/internal/scheduler"
	"github.com/samautomateai/tessivo/internal/service"
)

type fakeNotifier struct {
	mu    sync.Mutex
	notes []notify.Note
	err   error
}

func (f *fakeNotifier) Notify(_ context.Context, n notify.Note) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notes = append(f.notes, n)
	return f.err
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.notes)
}

func TestNotifyTestCommand(t *testing.T) {
	h := newHarness(t)
	n := &fakeNotifier{}
	h.env.Notifier = n
	if out := h.mustRun("notify", "test"); !strings.Contains(out, "sent a test notification") || n.count() != 1 {
		t.Fatalf("out %q notes %d", out, n.count())
	}
	if code := h.run("notify"); code != ExitUsage {
		t.Fatalf("usage: %d", code)
	}
	n.err = errors.New("no display")
	if code := h.run("notify", "test"); code != ExitError || !strings.Contains(h.errOut.String(), "no display") {
		t.Fatalf("failure: %d %s", code, h.errOut)
	}
}

func TestScheduleNotes(t *testing.T) {
	ev := func(kind, title, errText string) scheduler.Event {
		return scheduler.Event{Kind: kind, NotebookName: "Ops", Title: title, Error: errText}
	}
	cases := []struct {
		e    scheduler.Event
		body string
		ok   bool
	}{
		{ev(scheduler.EventFinished, "Week 47", ""), "Week 47 is ready", true},
		{ev(scheduler.EventFinished, "", ""), "Scheduled run finished", true},
		{ev(scheduler.EventFailed, "", "401 Bearer sk-abcdefghijklmnop"), "", true},
		{ev(scheduler.EventFailed, "", ""), "Scheduled run failed", true},
		{ev(scheduler.EventSkipped, "", ""), "A missed scheduled run was skipped", true},
		{ev(scheduler.EventStarted, "", ""), "", false},
	}
	for _, c := range cases {
		n, ok := scheduleNote(c.e)
		if ok != c.ok || (c.body != "" && n.Body != c.body) {
			t.Errorf("%s: %+v %v", c.e.Kind, n, ok)
		}
		if ok && n.Title != "Tessivo · Ops" {
			t.Errorf("title %q", n.Title)
		}
		if strings.Contains(n.Body, "sk-abcdefghijklmnop") {
			t.Errorf("secret not redacted: %q", n.Body)
		}
	}
}

func TestStartNotes(t *testing.T) {
	h := newHarness(t)
	n := &fakeNotifier{}
	h.env.Notifier = n
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startNotes(ctx, false, h.env)(scheduler.Event{Kind: scheduler.EventFinished})
	hook := startNotes(ctx, true, h.env)
	hook(scheduler.Event{Kind: scheduler.EventStarted})
	hook(scheduler.Event{Kind: scheduler.EventFinished, NotebookName: "Ops"})
	deadline := time.Now().Add(2 * time.Second)
	for n.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n.count() != 1 {
		t.Fatalf("want one note, got %d", n.count())
	}
}

func TestRunNote(t *testing.T) {
	cases := []struct {
		e    service.RunEvent
		body string
		ok   bool
	}{
		{
			service.RunEvent{Kind: service.RunApprovalNeeded, NotebookName: "Ops", Tool: "grep"},
			"Approval needed: grep", true,
		},
		{
			service.RunEvent{
				Kind: service.RunFailed, NotebookName: "Ops", ScheduleID: "s1", Error: "Bearer sk-abcdefghijklmnop",
			},
			"", true,
		},
		{service.RunEvent{Kind: service.RunFailed, NotebookName: "Ops"}, "", false},
		{service.RunEvent{Kind: service.RunFinished, NotebookName: "Ops"}, "", false},
	}
	for _, c := range cases {
		n, ok := runNote(c.e)
		if ok != c.ok || (c.body != "" && n.Body != c.body) {
			t.Errorf("%s: %+v %v", c.e.Kind, n, ok)
		}
		if ok && n.Title != "Tessivo · Ops" {
			t.Errorf("title %q", n.Title)
		}
		if strings.Contains(n.Body, "sk-abcdefghijklmnop") {
			t.Errorf("secret not redacted: %q", n.Body)
		}
	}
}
