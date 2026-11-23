package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Installers (ADR 0008). Each runs on the CI runner of its OS, after the
// bundle was staged (and signed, when the secrets exist):
//
//	linux    .deb (pure Go) and .AppImage (appimagetool)
//	windows  Setup.exe (Inno Setup's iscc)
//	macos    .dmg (hdiutil)

// runner runs a packaging tool; tests replace it.
type runner func(ctx context.Context, dir, name string, args ...string) error

// osRun runs name with args in dir, output to stderr. The tools and their
// arguments are fixed by this program.
func osRun(ctx context.Context, dir, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: fixed packaging tools
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stderr, os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(name), err)
	}
	return nil
}

// iconPath is the app icon in the repository (256 px PNG).
const iconPath = "desktop/macos/Runner/Assets.xcassets/AppIcon.appiconset/app_icon_256.png"

// installer builds the platform's installers from its staged bundle into dist.
func (a *app) installer(ctx context.Context, version, dist, platform, arch, tool string) error {
	bundle, err := a.bundlePath(platform, arch)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0o750); err != nil { //nolint:gosec // G703: dist is the -out flag
		return fmt.Errorf("create %s: %w", dist, err)
	}
	dist, err = filepath.Abs(dist)
	if err != nil {
		return fmt.Errorf("dist: %w", err)
	}
	base := fmt.Sprintf("tessivo-desktop_%s_%s_%s", version, platform, arch)
	switch platform {
	case "linux":
		return a.linuxInstallers(ctx, bundle, dist, base, version, arch, tool)
	case "windows":
		return a.innoSetup(ctx, bundle, dist, base, version, arch, tool)
	default:
		return a.dmg(ctx, bundle, dist, base)
	}
}

func (a *app) linuxInstallers(ctx context.Context, bundle, dist, base, version, arch, tool string) error {
	icon, err := a.readRepo(iconPath)
	if err != nil {
		return err
	}
	if err := writeDeb(filepath.Join(dist, base+".deb"), bundle, version, arch, icon); err != nil {
		return err
	}
	a.say("built %s.deb", base)
	if tool == "" {
		tool = "appimagetool"
	}
	appDir, err := os.MkdirTemp("", "tessivo-appdir-")
	if err != nil {
		return fmt.Errorf("appdir: %w", err)
	}
	defer func() { _ = os.RemoveAll(appDir) }()
	if err := stageAppDir(bundle, appDir, icon); err != nil {
		return err
	}
	out := filepath.Join(dist, base+".AppImage")
	if err := a.run(ctx, dist, tool, "--no-appstream", appDir, out); err != nil {
		return err
	}
	a.say("built %s.AppImage", base)
	return nil
}

// appRun starts the app from wherever the AppImage mounted it.
const appRun = "#!/bin/sh\nHERE=\"$(dirname \"$(readlink -f \"$0\")\")\"\nexec \"$HERE/" + appBinary + "\" \"$@\"\n"

// stageAppDir lays out an AppDir: the bundle, AppRun, the desktop entry and
// the icon at its root, as appimagetool expects.
func stageAppDir(bundle, dir string, icon []byte) error {
	if err := copyTree(bundle, dir); err != nil {
		return err
	}
	files := []struct {
		name string
		body []byte
		mode os.FileMode
	}{
		{"AppRun", []byte(appRun), 0o755},
		{"tessivo.desktop", []byte(desktopEntry("AppRun")), 0o644},
		{"tessivo.png", icon, 0o644},
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, f.body, f.mode); err != nil {
			return fmt.Errorf("appdir %s: %w", f.name, err)
		}
	}
	return nil
}

// copyTree copies src into dst, keeping modes and symlinks.
func copyTree(src, dst string) error {
	return walkFiles(src, "", func(p, name string, d os.DirEntry) error {
		target := filepath.Join(dst, filepath.FromSlash(name))
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755) //nolint:gosec // G301: bundle directories are public
		case d.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			return os.Symlink(link, target)
		}
		in, err := os.ReadFile(p) //nolint:gosec // G304: files of the bundle being packed
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := os.WriteFile(target, in, info.Mode().Perm()); err != nil { //nolint:gosec // G703: staging dir
			return fmt.Errorf("%s: %w", name, err)
		}
		return nil
	})
}

// innoArch is Inno Setup's architecture identifier.
var innoArch = map[string]string{archAMD64: "x64compatible", archARM64: archARM64}

// innoAppID keeps upgrades and uninstall tied to one product; never change it.
const innoAppID = "{{6C1E0B57-3C1F-4E0B-9B57-7A4C9E2D5F31}"

// innoScript is the Inno Setup script for a per-user install (no admin
// rights unless the user picks all users) with a Start menu entry.
func innoScript(bundle, dist, base, version, arch string) string {
	b := strings.ReplaceAll(bundle, "/", `\`)
	lines := []string{
		"[Setup]",
		"AppId=" + innoAppID,
		"AppName=Tessivo",
		"AppVersion=" + version,
		"AppPublisher=Tessivo",
		"AppPublisherURL=" + homepage,
		`DefaultDirName={autopf}\Tessivo`,
		"DefaultGroupName=Tessivo",
		"DisableProgramGroupPage=yes",
		"PrivilegesRequired=lowest",
		"PrivilegesRequiredOverridesAllowed=dialog",
		"ArchitecturesAllowed=" + innoArch[arch],
		"ArchitecturesInstallIn64BitMode=" + innoArch[arch],
		"OutputDir=" + strings.ReplaceAll(dist, "/", `\`),
		"OutputBaseFilename=" + base + "_setup",
		`LicenseFile=` + b + `\LICENSE`,
		`UninstallDisplayIcon={app}\` + appBinary + ".exe",
		"Compression=lzma2",
		"SolidCompression=yes",
		"WizardStyle=modern",
		"CloseApplications=yes",
		"",
		"[Tasks]",
		`Name: "desktopicon"; Description: "Create a desktop shortcut"; Flags: unchecked`,
		"",
		"[Files]",
		`Source: "` + b + `\*"; DestDir: "{app}"; Flags: recursesubdirs ignoreversion`,
		"",
		"[Icons]",
		`Name: "{group}\Tessivo"; Filename: "{app}\` + appBinary + `.exe"`,
		`Name: "{autodesktop}\Tessivo"; Filename: "{app}\` + appBinary + `.exe"; Tasks: desktopicon`,
		"",
		"[Run]",
		`Filename: "{app}\` + appBinary + `.exe"; Description: "Start Tessivo"; Flags: nowait postinstall skipifsilent`,
	}
	return strings.Join(lines, "\r\n") + "\r\n"
}

func (a *app) innoSetup(ctx context.Context, bundle, dist, base, version, arch, tool string) error {
	if tool == "" {
		tool = "iscc"
	}
	iss := filepath.Join(dist, base+".iss")
	if err := writePublic(iss, []byte(innoScript(bundle, dist, base, version, arch))); err != nil {
		return fmt.Errorf("write %s: %w", iss, err)
	}
	defer func() { _ = os.Remove(iss) }() //nolint:gosec // G703: iss is in the -out dir
	if err := a.run(ctx, dist, tool, "/Q", iss); err != nil {
		return err
	}
	a.say("built %s_setup.exe", base)
	return nil
}

// dmg puts the app, renamed Tessivo.app, and a link to /Applications in a
// compressed disk image. The bundle is moved for the build and moved back.
func (a *app) dmg(ctx context.Context, bundle, dist, base string) error {
	stage, err := os.MkdirTemp(filepath.Dir(bundle), "dmg-")
	if err != nil {
		return fmt.Errorf("dmg stage: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()
	app := filepath.Join(stage, "Tessivo.app")
	if err := os.Rename(bundle, app); err != nil {
		return fmt.Errorf("dmg stage: %w", err)
	}
	defer func() { _ = os.Rename(app, bundle) }()
	if err := os.Symlink("/Applications", filepath.Join(stage, "Applications")); err != nil {
		a.say("dmg: no Applications link (%v)", err)
	}
	out := filepath.Join(dist, base+".dmg")
	err = a.run(ctx, dist, "hdiutil", "create", "-volname", "Tessivo", "-srcfolder", stage,
		"-ov", "-format", "UDZO", out)
	if err != nil {
		return err
	}
	a.say("built %s.dmg", base)
	return nil
}
