package cli

import (
	"context"
	"os"
	"runtime"
	"time"
)

const goosWindows = "windows"

// exitWithParent cancels the returned context when the process that started
// us goes away, so a desktop app crash never leaves an orphaned core.
func exitWithParent(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	parent := os.Getppid()
	go func() {
		if runtime.GOOS == goosWindows {
			if p, err := os.FindProcess(parent); err == nil {
				_, _ = p.Wait()
				cancel()
			}
			return
		}
		watchParent(ctx, parent, os.Getppid, 2*time.Second, cancel)
	}()
	return ctx, cancel
}

// watchParent polls getppid; on Unix an orphan is re-parented, so a change
// means the original parent exited.
func watchParent(ctx context.Context, parent int, getppid func() int, every time.Duration, done func()) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if getppid() != parent {
				done()
				return
			}
		}
	}
}
