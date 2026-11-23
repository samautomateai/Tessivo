package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/atomicfile"
)

// sseWriter writes server-sent events and flushes each one.
type sseWriter struct {
	mu sync.Mutex
	w  http.ResponseWriter
	f  http.Flusher
}

func newSSE(w http.ResponseWriter) (*sseWriter, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return &sseWriter{w: w, f: f}, true
}

func (s *sseWriter) send(event string, data any) {
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, b)
	s.f.Flush()
}

func (s *sseWriter) comment(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.w, ": %s\n\n", text)
	s.f.Flush()
}

// Endpoint tells the desktop app how to reach the core. It is written 0600
// and removed on shutdown; the token never appears in argv or logs.
type Endpoint struct {
	Network string `json:"network"` // unix or tcp
	Address string `json:"address"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

// NewToken returns 32 random bytes, hex encoded.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Listen opens a Unix socket (0600, in a 0700 directory, replacing a stale
// socket) or a loopback TCP listener. Non-loopback TCP addresses are refused.
func Listen(ctx context.Context, network, address string) (net.Listener, error) {
	switch network {
	case "unix":
		return listenUnix(ctx, address)
	case "tcp":
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("listen address %q: %w", address, err)
		}
		if ip := net.ParseIP(host); host != localhost && (ip == nil || !ip.IsLoopback()) {
			return nil, fmt.Errorf("refusing to listen on non-loopback address %q", address)
		}
		ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("listen: %w", err)
		}
		return ln, nil
	default:
		return nil, fmt.Errorf("unknown network %q (want unix or tcp)", network)
	}
}

func listenUnix(ctx context.Context, path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("socket dir: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("%s exists and is not a socket", path)
		}
		d := net.Dialer{Timeout: 200 * time.Millisecond}
		if c, err := d.DialContext(ctx, "unix", path); err == nil {
			_ = c.Close()
			return nil, fmt.Errorf("another Tessivo core is already listening on %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("secure socket: %w", err)
	}
	return ln, nil
}

// WriteEndpoint writes the endpoint file atomically with 0600 permissions.
func WriteEndpoint(path string, ep Endpoint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("endpoint dir: %w", err)
	}
	b, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return fmt.Errorf("encode endpoint: %w", err)
	}
	if err := atomicfile.Write(path, b, 0o600); err != nil {
		return fmt.Errorf("endpoint: %w", err)
	}
	return nil
}

// Serve runs the HTTP server on ln until ctx is cancelled, then shuts down
// gracefully (waiting up to 10s for in-flight requests).
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shut); err != nil { //nolint:contextcheck // shutdown must outlive the cancelled ctx
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
