package mcp

// The reader/dispatch design follows Zero internal/mcp/client.go (commit
// 8288ae81bd6a, MIT; see doc.go): one reader goroutine routes responses to
// waiting callers by ID, and a dead reader fails every pending call.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/proctree"
)

// maxStderr is how much of a server's stderr is kept (the start of it).
const maxStderr = 64 << 10

// childEnvNames are the parent variables a server inherits; everything else
// (including credentials in the parent's environment) is left out.
var childEnvNames = []string{
	"PATH", "PATHEXT", "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "SYSTEMROOT", "WINDIR", "COMSPEC",
	"TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME",
}

type stdioConn struct {
	cmd    *exec.Cmd
	tree   *proctree.Tree // nil when the OS could not name the tree; then only cmd is killed
	stdin  io.WriteCloser
	stderr *capped

	writes  chan writeOp // the one writer goroutine drains it; bounded
	mu      sync.Mutex
	pending map[int64]chan message
	dead    error         // set once the reader stops
	exited  chan struct{} // closed when the process has been waited for
	closing sync.Once
}

func startStdio(ctx context.Context, s Server) (*stdioConn, error) {
	// The process lives until close, not until the context of whoever started it.
	//nolint:gosec // G204: the program is the server the user configured
	cmd := exec.CommandContext(context.WithoutCancel(ctx), s.Command, s.Args...)
	cmd.Env = childEnv(s.Env)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout: %w", err)
	}
	c := &stdioConn{
		cmd: cmd, stdin: stdin, stderr: &capped{limit: maxStderr},
		pending: map[int64]chan message{}, exited: make(chan struct{}), writes: make(chan writeOp, writeQueue),
	}
	cmd.Stderr = c.stderr
	proctree.Prepare(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", s.Command, err)
	}
	if c.tree, err = proctree.Attach(cmd); err != nil {
		c.tree = nil // still usable: close kills the direct process
	}
	go c.readLoop(stdout)
	go c.writeLoop()
	return c, nil
}

// childEnv is the allow-listed parent environment plus the server's own.
func childEnv(own map[string]string) []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		for _, allowed := range childEnvNames {
			if strings.EqualFold(name, allowed) {
				env = append(env, kv)
				break
			}
		}
	}
	for k, v := range own {
		env = append(env, k+"="+v)
	}
	return env
}

func (c *stdioConn) readLoop(stdout io.Reader) {
	r := newReader(stdout)
	var err error
	for {
		var m message
		if m, err = r.read(); err != nil {
			break
		}
		c.dispatch(m)
	}
	if !errors.Is(err, ErrTooLarge) {
		err = ErrServerExited
	}
	c.mu.Lock()
	c.dead = err
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if errors.Is(err, ErrTooLarge) {
		c.kill() // a flooding server is stopped at once
	}
	_ = c.cmd.Wait()
	if c.tree != nil {
		c.kill() // descendants the server left behind when it exited
		_ = c.tree.Release()
	}
	close(c.exited)
}

func (c *stdioConn) dispatch(m message) {
	if m.Method != "" {
		if m.ID != nil { // a request from the server: answer ping, refuse the rest
			c.reply(m)
		}
		return
	}
	id, err := strconv.ParseInt(string(m.ID), 10, 64)
	if err != nil {
		return
	}
	c.mu.Lock()
	ch, ok := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ok {
		ch <- m
	}
}

func (c *stdioConn) reply(req message) {
	resp := message{JSONRPC: jsonrpcVersion, ID: req.ID}
	if req.Method == "ping" {
		resp.Result = json.RawMessage(`{}`)
	} else {
		resp.Error = &rpcError{Code: -32601, Message: "method not supported"}
	}
	select {
	case c.writes <- writeOp{m: resp}:
	default: // the queue is full: a server flooding requests gets no more answers
	}
}

// writeQueue bounds the messages waiting to be written to the server.
const writeQueue = 32

type writeOp struct {
	m    message
	done chan error // nil for replies nobody waits on
}

// writeLoop is the only goroutine that writes to the server.
func (c *stdioConn) writeLoop() {
	for {
		select {
		case op := <-c.writes:
			err := writeMessage(c.stdin, op.m)
			if op.done != nil {
				op.done <- err
			}
		case <-c.exited:
			return
		}
	}
}

// write queues m and waits until it is written. A server that stops reading
// its input past ctx's deadline is stopped: nothing could reach it anymore.
func (c *stdioConn) write(ctx context.Context, m message) error {
	done := make(chan error, 1)
	select {
	case c.writes <- writeOp{m: m, done: done}:
	case <-ctx.Done():
		return c.stall(ctx.Err())
	case <-c.exited:
		return c.deadOr(ErrServerExited)
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return c.stall(ctx.Err())
	case <-c.exited:
		return c.deadOr(ErrServerExited)
	}
}

// stall marks the server as gone and kills it.
func (c *stdioConn) stall(cause error) error {
	c.mu.Lock()
	if c.dead == nil {
		c.dead = fmt.Errorf("mcp: server stopped reading its input: %w", cause)
	}
	err := c.dead
	c.mu.Unlock()
	c.kill()
	return err
}

func (c *stdioConn) roundTrip(ctx context.Context, m message) (message, error) {
	id, _ := strconv.ParseInt(string(m.ID), 10, 64)
	ch := make(chan message, 1)
	c.mu.Lock()
	if c.dead != nil {
		err := c.dead
		c.mu.Unlock()
		return message{}, err
	}
	c.pending[id] = ch
	c.mu.Unlock()
	if err := c.write(ctx, m); err != nil {
		c.forget(id)
		return message{}, c.deadOr(err)
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			return message{}, c.deadOr(ErrServerExited)
		}
		return resp, nil
	case <-ctx.Done():
		c.forget(id)
		return message{}, fmt.Errorf("mcp: no answer: %w", ctx.Err())
	}
}

func (c *stdioConn) forget(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// deadOr returns why the reader stopped, if it has, else err.
func (c *stdioConn) deadOr(err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dead != nil {
		return c.dead
	}
	return err
}

func (c *stdioConn) notify(ctx context.Context, m message) error {
	if err := c.write(ctx, m); err != nil {
		return c.deadOr(err)
	}
	return nil
}

// close closes stdin (servers exit on EOF), then kills the process if it is
// still running after closeGrace, with every process it started.
func (c *stdioConn) close() error {
	c.closing.Do(func() {
		c.mu.Lock()
		if c.dead == nil {
			c.dead = errClosed
		}
		c.mu.Unlock()
		_ = c.stdin.Close()
		select {
		case <-c.exited:
			return
		case <-time.After(closeGrace):
		}
		c.kill()
		select { // a child that inherited stdout can keep the pipe open; do not wait forever
		case <-c.exited:
		case <-time.After(closeGrace):
		}
	})
	return nil
}

// withStderr adds the start of the server's stderr to a startup error.
func (c *stdioConn) withStderr(err error) error {
	if s := strings.TrimSpace(c.stderr.String()); s != "" {
		return fmt.Errorf("%w; server said: %s", err, s)
	}
	return err
}

// capped keeps the first limit bytes written to it.
type capped struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (b *capped) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.limit - len(b.buf); room > 0 {
		b.buf = append(b.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (b *capped) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// kill stops the server and, where the OS can name them, the processes it started.
func (c *stdioConn) kill() {
	if c.tree != nil {
		_ = c.tree.Kill()
		return
	}
	_ = c.cmd.Process.Kill()
}
