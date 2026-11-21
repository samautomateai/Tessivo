package llm

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// newHTTPClient returns a hardened client for provider APIs:
//   - TLS 1.2 or newer; proxy from the environment;
//   - bounded dial, TLS handshake and time-to-first-byte;
//   - no overall timeout (long streams are fine) — idle streams are cut by idleReader;
//   - redirects are refused, so custom headers and keys never reach another host.
func newHTTPClient(timeout time.Duration) *http.Client {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewHTTPClient is newHTTPClient for other packages (the MCP transport).
func NewHTTPClient(timeout time.Duration) *http.Client { return newHTTPClient(timeout) }

// ErrIdleTimeout reports a stream that stopped sending data.
var ErrIdleTimeout = errors.New("stream idle timeout")

// idleReader cancels the request when no bytes arrive for idle. It wraps a
// response body; Read resets the timer on every chunk.
type idleReader struct {
	r      io.ReadCloser
	idle   time.Duration
	cancel context.CancelCauseFunc
	mu     sync.Mutex
	timer  *time.Timer
}

func newIdleReader(r io.ReadCloser, idle time.Duration, cancel context.CancelCauseFunc) *idleReader {
	ir := &idleReader{r: r, idle: idle, cancel: cancel}
	ir.timer = time.AfterFunc(idle, func() { cancel(ErrIdleTimeout) })
	return ir
}

func (ir *idleReader) Read(p []byte) (int, error) {
	n, err := ir.r.Read(p)
	if n > 0 {
		ir.mu.Lock()
		ir.timer.Reset(ir.idle)
		ir.mu.Unlock()
	}
	return n, err //nolint:wrapcheck // io.Reader contract: return the underlying error (io.EOF) unchanged
}

func (ir *idleReader) Close() error {
	ir.timer.Stop()
	return ir.r.Close() //nolint:wrapcheck // io.Closer passthrough
}
