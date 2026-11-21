package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// build is one `go build` of a package for a target. The program and
// arguments are fixed by this tool.
type build struct {
	dir, pkg, out, goos, goarch, ldflags string
}

const versionVar = "github.com/samautomateai/tessivo/internal/cli.Version"

func newBuild(root, version string, t target, out string) build {
	return build{
		dir: root, pkg: "./cmd/tess", out: out, goos: t.goos, goarch: t.goarch,
		ldflags: "-s -w -X " + versionVar + "=" + version,
	}
}

type goBuilder interface {
	build(ctx context.Context, b build) error
}

type osGo struct{}

// build runs go build with cgo off, so every target cross-compiles from any OS.
func (osGo) build(ctx context.Context, b build) error {
	if err := os.MkdirAll(filepath.Dir(b.out), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(b.out), err)
	}
	c := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags", b.ldflags, "-o", b.out, b.pkg)
	c.Dir = b.dir
	c.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+b.goos, "GOARCH="+b.goarch)
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("go build %s/%s: %w", b.goos, b.goarch, err)
	}
	return nil
}
