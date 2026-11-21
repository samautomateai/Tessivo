package cli

import (
	"context"
	"fmt"

	"github.com/samautomateai/tessivo/internal/notify"
	"github.com/samautomateai/tessivo/internal/scheduler"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

// cmdNotify sends a test notification so users can check their system.
func cmdNotify(ctx context.Context, args []string, env Env) error {
	if len(args) != 1 || args[0] != "test" {
		return usagef("notify: want 'tess notify test'")
	}
	n := env.Notifier
	if n == nil {
		n = notify.OS()
	}
	if err := n.Notify(ctx, notify.Note{Title: "Tessivo", Body: "Notifications work."}); err != nil {
		return fmt.Errorf("notification failed: %w", err)
	}
	_, err := fmt.Fprintln(env.Stdout, "sent a test notification")
	return wrapWrite(err)
}

// scheduleNote turns a scheduler event into a notification. Starts are not
// worth interrupting anyone for; errors are redacted before display.
func scheduleNote(e scheduler.Event) (notify.Note, bool) {
	title := "Tessivo · " + e.NotebookName
	switch e.Kind {
	case scheduler.EventFinished:
		body := "Scheduled run finished"
		if e.Title != "" {
			body = e.Title + " is ready"
		}
		return notify.Note{Title: title, Body: body}, true
	case scheduler.EventFailed:
		body := "Scheduled run failed"
		if e.Error != "" {
			body += ": " + secrets.RedactPatterns(e.Error)
		}
		return notify.Note{Title: title, Body: body}, true
	case scheduler.EventSkipped:
		return notify.Note{Title: title, Body: "A missed scheduled run was skipped"}, true
	default:
		return notify.Note{}, false
	}
}

// runNote is the OS notification for an agent run event: always when a run
// needs approval, and when a scheduled run fails.
func runNote(e service.RunEvent) (notify.Note, bool) {
	title := "Tessivo · " + e.NotebookName
	switch {
	case e.Kind == service.RunApprovalNeeded:
		return notify.Note{Title: title, Body: "Approval needed: " + e.Tool}, true
	case e.Kind == service.RunFailed && e.ScheduleID != "":
		body := "Scheduled agent run failed"
		if e.Error != "" {
			body += ": " + secrets.RedactPatterns(e.Error)
		}
		return notify.Note{Title: title, Body: body}, true
	}
	return notify.Note{}, false
}
