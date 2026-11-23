// Package proctree stops a child process together with everything it started.
//
// Killing only the direct child leaves its own children running (an MCP server
// started through npx, uvx or a shell script is a tree). Each OS has its own way
// to name the tree (ADR 0006): a process group on Unix, a Job Object on Windows.
// Elsewhere only the direct child is killed.
//
// Use: Prepare(cmd) before cmd.Start, Attach(cmd) after it, Kill to stop the
// tree and Release once the process has been waited for.
package proctree

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
)

// errNotStarted is returned by Attach for a command that has not started.
var errNotStarted = errors.New("proctree: command not started")

// Tree is a started process and its descendants.
type Tree struct {
	cmd *exec.Cmd
	os  osTree

	mu       sync.Mutex
	released bool // after Release the OS handle may name something else
}

// osTree is the per-OS part, in proctree_unix.go, proctree_windows.go and proctree_other.go.
type osTree interface {
	kill() error
	release() error
}

// Prepare sets cmd up so its descendants can be found later. Call it before cmd.Start.
func Prepare(cmd *exec.Cmd) { prepare(cmd) }

// Attach returns the tree of the started cmd.
func Attach(cmd *exec.Cmd) (*Tree, error) {
	if cmd.Process == nil {
		return nil, errNotStarted
	}
	t, err := attach(cmd)
	if err != nil {
		return nil, fmt.Errorf("proctree: %w", err)
	}
	return &Tree{cmd: cmd, os: t}, nil
}

// Kill stops the process and everything it started. Killing a tree that has
// already exited is not an error.
func (t *Tree) Kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.released {
		return nil
	}
	err := t.os.kill()
	_ = t.cmd.Process.Kill() // the direct child, in case the OS call missed it
	if err != nil {
		return fmt.Errorf("proctree: kill: %w", err)
	}
	return nil
}

// Release frees what the OS holds for the tree. On Windows it also stops any
// descendant still running, so call it only once the tree should be gone.
func (t *Tree) Release() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.released {
		return nil
	}
	t.released = true
	if err := t.os.release(); err != nil {
		return fmt.Errorf("proctree: release: %w", err)
	}
	return nil
}
