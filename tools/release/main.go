// Command release builds downloadable release archives:
//
//	go run ./tools/release cli -version v1.2.3 -out dist
//	    tess for every OS and architecture (tess_<version>_<os>_<arch>.zip|tar.gz)
//	go run ./tools/release desktop -version v1.2.3 -out dist -platform linux -arch amd64
//	    the Flutter bundle built by `flutter build <platform> --release`, with tess beside the app
//	go run ./tools/release sums -dir dist
//	    SHA256SUMS for every file in dist
//
// Windows archives are .zip; macOS and Linux archives are .tar.gz, which keep
// file modes and the symlinks inside macOS app bundles. The per-platform
// table describes target bundles; nothing depends on the OS this runs on.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, osGo{}))
}

// target is one GOOS/GOARCH pair for the CLI.
type target struct{ goos, goarch string }

var cliTargets = []target{
	{goosLinux, archAMD64},
	{goosLinux, archARM64},
	{goosDarwin, archAMD64},
	{goosDarwin, archARM64},
	{goosWindows, archAMD64},
	{goosWindows, archARM64},
}

// desktopTarget says where Flutter puts a platform's release bundle (relative
// to the repo, with "{arch}" for x64/arm64) and where tess goes inside it, so
// the app finds it next to its own executable.
type desktopTarget struct {
	goos, bundle, tessDir string
}

var desktopTargets = map[string]desktopTarget{
	"linux":   {goos: goosLinux, bundle: "desktop/build/linux/{arch}/release/bundle"},
	"windows": {goos: goosWindows, bundle: "desktop/build/windows/{arch}/runner/Release"},
	"macos": {
		goos: goosDarwin, bundle: "desktop/build/macos/Build/Products/Release/tessivo_desktop.app",
		tessDir: "Contents/MacOS",
	},
}

// flutterArch is Flutter's name for a Go architecture in build paths.
var flutterArch = map[string]string{archAMD64: "x64", archARM64: "arm64"}

type app struct {
	root    string // repository root
	goBuild func(ctx context.Context, b build) error
	out     io.Writer
}

func (a *app) say(format string, args ...any) { _, _ = fmt.Fprintf(a.out, format+"\n", args...) }

// cli builds and archives tess for every target.
func (a *app) cli(ctx context.Context, version, dist string) error {
	if err := os.MkdirAll(dist, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dist, err)
	}
	for _, t := range cliTargets {
		name := fmt.Sprintf("tess_%s_%s_%s", version, t.goos, t.goarch)
		stage, err := os.MkdirTemp("", "tess-release-")
		if err != nil {
			return fmt.Errorf("staging dir: %w", err)
		}
		dir := filepath.Join(stage, name)
		err = a.stageCLI(ctx, version, t, dir)
		if err == nil {
			err = archive(dir, filepath.Join(dist, name+ext(t.goos)), t.goos)
		}
		_ = os.RemoveAll(stage)
		if err != nil {
			return err
		}
		a.say("built %s", name+ext(t.goos))
	}
	return nil
}

func (a *app) stageCLI(ctx context.Context, version string, t target, dir string) error {
	if err := a.goBuild(ctx, newBuild(a.root, version, t, filepath.Join(dir, exe("tess", t.goos)))); err != nil {
		return err
	}
	readme, err := os.ReadFile(filepath.Join(a.root, "README.md"))
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}
	if err := writePublic(filepath.Join(dir, "README.md"), readme); err != nil {
		return fmt.Errorf("stage README.md: %w", err)
	}
	return nil
}

// desktop adds tess to the platform's Flutter bundle and archives the bundle.
func (a *app) desktop(ctx context.Context, version, dist, platform, arch string) error {
	dt, ok := desktopTargets[platform]
	fa, archOK := flutterArch[arch]
	if !ok || !archOK {
		return fmt.Errorf("unknown platform %q or arch %q (want linux|windows|macos, amd64|arm64)", platform, arch)
	}
	bundle := filepath.Join(a.root, filepath.FromSlash(strings.ReplaceAll(dt.bundle, "{arch}", fa)))
	if _, err := os.Stat(bundle); err != nil {
		return fmt.Errorf("no bundle at %s; run: cd desktop && flutter build %s --release", bundle, platform)
	}
	t := target{dt.goos, arch}
	tess := filepath.Join(bundle, filepath.FromSlash(dt.tessDir), exe("tess", dt.goos))
	if err := a.goBuild(ctx, newBuild(a.root, version, t, tess)); err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dist, err)
	}
	name := fmt.Sprintf("tessivo-desktop_%s_%s_%s", version, platform, arch)
	if err := archiveAs(bundle, name, filepath.Join(dist, name+ext(dt.goos)), dt.goos); err != nil {
		return err
	}
	a.say("built %s", name+ext(dt.goos))
	return nil
}

func ext(goos string) string {
	if goos == goosWindows {
		return ".zip"
	}
	return ".tar.gz"
}

func exe(name, goos string) string {
	if goos == goosWindows {
		return name + ".exe"
	}
	return name
}

func run(ctx context.Context, args []string, stderr io.Writer, g goBuilder) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: release cli|desktop|sums [flags]")
		return 2
	}
	fs := flag.NewFlagSet("release "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	version := fs.String("version", "", "version to embed and name archives with, e.g. v1.2.3")
	out := fs.String("out", "dist", "output directory")
	dir := fs.String("dir", "", "sums: directory of archives")
	platform := fs.String("platform", "", "desktop: linux, windows or macos")
	arch := fs.String("arch", archAMD64, "desktop: amd64 or arm64")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "release:", err)
		return 1
	}
	a := &app{root: root, goBuild: g.build, out: stderr}
	switch {
	case args[0] == "cli" && *version != "":
		err = a.cli(ctx, *version, *out)
	case args[0] == "desktop" && *version != "" && *platform != "":
		err = a.desktop(ctx, *version, *out, *platform, *arch)
	case args[0] == "sums" && *dir != "":
		err = sums(*dir)
	default:
		_, _ = fmt.Fprintln(stderr, "usage: release cli -version V | desktop -version V -platform P | sums -dir D")
		return 2
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "release:", err)
		return 1
	}
	return 0
}

const (
	goosWindows = "windows"
	goosLinux   = "linux"
	goosDarwin  = "darwin"
	archAMD64   = "amd64"
	archARM64   = "arm64"
)
