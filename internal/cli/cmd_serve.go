package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/samautomateai/tessivo/internal/scheduler"
	"github.com/samautomateai/tessivo/internal/server"
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
	api.Events = server.NewBroker()
	schedDone := make(chan struct{})
	if *noSched {
		close(schedDone)
	} else {
		sched := &scheduler.Scheduler{Svc: svc, OnEvent: func(e scheduler.Event) { api.Events.Publish(e) }}
		go func() {
			defer close(schedDone)
			_ = sched.Run(ctx)
		}()
	}
	err = server.Serve(ctx, ln, api.Handler())
	<-schedDone
	return err
}
