package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/logfile"
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
	*addr = defaultAddr(dir, *network, *addr)
	f, err := serveConfig(env)
	if err != nil {
		return err
	}
	svc, logw, err := serveService(f, env)
	if err != nil {
		return err
	}
	defer func() { _ = logw.Close() }()
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
	api.SetLogger(slog.New(slog.NewTextHandler(logw, nil)))
	api.Background = ctx // resumed runs stop (interrupted) when the core stops
	schedDone := startBackground(ctx, svc, api, !*noSched, *notes, env)
	go svc.RefreshPacks(ctx) // git packs catch up at start; each refresh is time-bounded
	err = server.Serve(ctx, ln, api.Handler())
	<-schedDone
	api.Wait()
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
		api.Logger().Info("event", "kind", e.Kind, "run", e.RunID) // no names or content
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
	svc.StartSummaries(ctx) // research summaries run while the core runs
	if err := svc.Recover(); err != nil {
		_, _ = fmt.Fprintf(env.Stderr, "tessivo: recovering agent runs: %v\n", err)
	}
	return startScheduler(ctx, svc, api, sched, send)
}

// serveService is the service behind the local API: the desktop is its
// actor, API keys set from the desktop go to the OS keychain, and the core
// logs to <config dir>/logs/core.log (the caller closes the log).
func serveService(f config.File, env Env) (*service.Service, *logfile.Writer, error) {
	svc, err := newService(f, env)
	if err != nil {
		return nil, nil, err
	}
	svc.Actor = "desktop"
	svc.SecretStore = env.Keychain
	svc.LogPath = logPath(svc.ConfigPath)
	logw, err := logfile.Open(svc.LogPath, logMaxBytes, logKeep)
	if err != nil {
		return nil, nil, err
	}
	return svc, logw, nil
}

// Core log size: 5 MiB per file, 3 old files kept.
const (
	logMaxBytes = 5 << 20
	logKeep     = 3
)

// logPath is the core log next to the config file.
func logPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "logs", "core.log")
}

// defaultAddr is the listen address when none is given.
func defaultAddr(dir, network, addr string) string {
	switch {
	case addr != "":
		return addr
	case network == "tcp":
		return "127.0.0.1:0"
	default:
		return filepath.Join(dir, "tessivo.sock")
	}
}

// serveConfig is the config file, or the defaults on a first run: the desktop
// app starts the core before anyone has set anything up, and its first-run
// screens guide the user from there. The first settings change writes the file.
func serveConfig(env Env) (config.File, error) {
	p, err := configPath(env)
	if err != nil {
		return config.File{}, err
	}
	f, err := config.Load(p)
	if errors.Is(err, os.ErrNotExist) {
		_, _ = fmt.Fprintf(env.Stderr, "tess: no config at %s yet; starting with the defaults\n", p)
		return config.Default(), nil
	}
	if err != nil {
		return config.File{}, fmt.Errorf("config %s: %w", p, err)
	}
	return f, nil
}
