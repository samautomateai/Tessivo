package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/samautomateai/tessivo/internal/notify"
	"github.com/samautomateai/tessivo/internal/scheduler"
	"github.com/samautomateai/tessivo/internal/server"
	"github.com/samautomateai/tessivo/internal/service"
)

// runtimeDir holds the socket and endpoint file (0700).
func runtimeDir(env Env) (string, error) {
	p, err := configPath(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "run"), nil
}

func cmdServe(ctx context.Context, args []string, env Env) error {
	dir, err := runtimeDir(env)
	if err != nil {
		return err
	}
	defNet := "unix"
	if runtime.GOOS == goosWindows {
		defNet = "tcp"
	}
	fs := newFlags("serve", env)
	network := fs.String("network", defNet, "unix (macOS/Linux) or tcp (loopback only)")
	addr := fs.String("addr", "", "socket path or host:port (default: runtime dir socket, or 127.0.0.1:0)")
	epPath := fs.String("endpoint-file", filepath.Join(dir, "endpoint.json"), "where to write connection details")
	withParent := fs.Bool("exit-with-parent", false, "stop when the launching process (the desktop app) exits")
	noSched := fs.Bool("no-scheduler", false, "do not run notebook schedules")
	notes := fs.Bool("notify", false, "show OS notifications when scheduled runs finish, fail or are skipped")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if *withParent {
		var cancel context.CancelFunc
		ctx, cancel = exitWithParent(ctx)
		defer cancel()
	}
	if *addr == "" {
		*addr = filepath.Join(dir, "tessivo.sock")
		if *network == "tcp" {
			*addr = "127.0.0.1:0"
		}
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	svc.Actor = "desktop"
	token, err := server.NewToken()
	if err != nil {
		return err
	}
	ln, err := server.Listen(ctx, *network, *addr)
	if err != nil {
		return err
	}
	ep := server.Endpoint{
		Network: *network, Address: ln.Addr().String(), Token: token, PID: os.Getpid(), Version: Version,
	}
	if err := server.WriteEndpoint(*epPath, ep); err != nil {
		_ = ln.Close()
		return err
	}
	defer func() { _ = os.Remove(*epPath) }()
	_, _ = fmt.Fprintf(env.Stderr, "tessivo core listening on %s %s (endpoint: %s)\n", *network, ep.Address, *epPath)
	api := server.New(svc, token, Version)
	schedDone := startBackground(ctx, svc, api, !*noSched, *notes, env)
	err = server.Serve(ctx, ln, api.Handler())
	<-schedDone
	return err
}

// startScheduler runs notebook schedules in the background (unless disabled)
// and returns a channel closed when the scheduler has stopped.
func startScheduler(ctx context.Context, svc *service.Service, api *server.Server, enabled bool,
	send func(notify.Note),
) <-chan struct{} {
	done := make(chan struct{})
	if !enabled {
		close(done)
		return done
	}
	sched := &scheduler.Scheduler{Svc: svc, OnEvent: func(e scheduler.Event) {
		api.Events.Publish(e)
		if note, ok := scheduleNote(e); ok {
			send(note)
		}
	}}
	go func() {
		defer close(done)
		_ = sched.Run(ctx)
	}()
	return done
}

// startNotes returns the scheduler hook for OS notifications: a no-op unless
// enabled, otherwise a non-blocking send to a rate-limited dispatcher.
func startNotes(ctx context.Context, enabled bool, env Env) func(scheduler.Event) {
	send := noteSender(ctx, enabled, env)
	return func(e scheduler.Event) {
		if note, ok := scheduleNote(e); ok {
			send(note)
		}
	}
}

// noteSender starts one rate-limited dispatcher shared by scheduler and run
// events; disabled, it drops every note.
func noteSender(ctx context.Context, enabled bool, env Env) func(notify.Note) {
	if !enabled {
		return func(notify.Note) {}
	}
	n := env.Notifier
	if n == nil {
		n = notify.OS()
	}
	d := &notify.Dispatcher{N: n, OnErr: func(err error) {
		_, _ = fmt.Fprintf(env.Stderr, "tessivo: %v\n", err)
	}}
	go d.Run(ctx)
	return func(note notify.Note) { d.Send(note) }
}

// wireRunEvents publishes agent run events on the API's event stream and
// notifies when a run needs approval.
func wireRunEvents(svc *service.Service, api *server.Server, send func(notify.Note)) {
	svc.OnRunEvent = func(e service.RunEvent) {
		api.Events.Publish(e)
		if note, ok := runNote(e); ok {
			send(note)
		}
	}
}

// startBackground wires events and notifications, recovers agent runs left by
// a previous core, and starts the scheduler.
func startBackground(ctx context.Context, svc *service.Service, api *server.Server, sched, notes bool, env Env,
) <-chan struct{} {
	api.Events = server.NewBroker()
	send := noteSender(ctx, notes, env)
	wireRunEvents(svc, api, send)
	if err := svc.Recover(); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "tessivo: recovering agent runs: %v\n", err)
	}
	return startScheduler(ctx, svc, api, sched, send)
}
