//go:build unix

package proctree

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// prepare puts the child in a process group of its own, so the group id names
// it and every descendant that does not leave the group.
func prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

type groupTree struct{ pgid int }

func attach(cmd *exec.Cmd) (osTree, error) {
	return groupTree{pgid: cmd.Process.Pid}, nil
}

func (g groupTree) kill() error {
	err := syscall.Kill(-g.pgid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) { // the group is already gone
		return nil
	}
	if err != nil {
		return fmt.Errorf("kill group: %w", err)
	}
	return nil
}

func (groupTree) release() error { return nil }
