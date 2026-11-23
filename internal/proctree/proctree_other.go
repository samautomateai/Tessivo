//go:build !unix && !windows

package proctree

import "os/exec"

// On other systems only the direct child is stopped (Tree.Kill kills it).
func prepare(*exec.Cmd) {}

type directTree struct{}

func attach(*exec.Cmd) (osTree, error) { return directTree{}, nil }

func (directTree) kill() error    { return nil }
func (directTree) release() error { return nil }
