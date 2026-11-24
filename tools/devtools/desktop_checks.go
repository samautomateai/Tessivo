package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// makeSymlink is os.Symlink; a variable so tests can make it fail.
var makeSymlink = os.Symlink

// checkSymlinks tries what Flutter does when it builds an app with plugins:
// create a symlink. Windows allows that only with Developer Mode on (or as an
// administrator), so this is where `flutter build windows` would fail.
func (a *app) checkSymlinks() result {
	res := result{name: "symlinks", st: ok, detail: "can be created (Flutter plugins need them)"}
	dir, err := os.MkdirTemp("", "tessivo-doctor-")
	if err != nil {
		res.st, res.detail = warn, "could not test: "+firstLine(err.Error())
		return res
	}
	defer func() { _ = os.RemoveAll(dir) }()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		res.st, res.detail = warn, "could not test: "+firstLine(err.Error())
		return res
	}
	if err := makeSymlink(target, filepath.Join(dir, "link")); err != nil {
		res.st, res.detail = warn, "cannot be created, so the desktop app will not build: "+firstLine(err.Error())
		res.hint = "turn on symlink support for your account and run task doctor again"
		if a.goos == "windows" { // approved per-OS text, like flutterInstallSteps
			res.hint = "turn on Developer Mode: run `start ms-settings:developers`, switch it on, then task doctor"
		}
	}
	return res
}

// compilerLine is how CMake records the C++ compiler it found.
var compilerLine = regexp.MustCompile(`set\(CMAKE_CXX_COMPILER "([^"]+)"\)`)

// checkBuildCache finds desktop builds whose CMake cache names a compiler
// that no longer exists (a Visual Studio, Xcode or clang update removed it);
// CMake then refuses to configure. With fix, the platform's generated build
// folder is deleted and the next flutter build starts clean.
func (a *app) checkBuildCache(fix bool) result {
	res := result{name: "desktop build", st: ok, detail: "no stale build cache"}
	build := filepath.Join(a.root, "desktop", "build")
	stale, err := staleBuilds(build)
	if err != nil {
		res.st, res.detail = warn, "could not read desktop/build: "+firstLine(err.Error())
		return res
	}
	if len(stale) == 0 {
		return res
	}
	if !fix {
		res.st = warn
		res.detail = "the build cache names a compiler that is gone: " + strings.Join(stale, ", ")
		res.hint = "run task doctor to delete those generated build folders (desktop/build/<platform>)"
		return res
	}
	for _, p := range stale {
		if err := os.RemoveAll(filepath.Join(build, p)); err != nil {
			res.st, res.detail = warn, fmt.Sprintf("could not delete desktop/build/%s: %s", p, firstLine(err.Error()))
			return res
		}
	}
	res.detail = "deleted the stale build cache of " + strings.Join(stale, ", ") + " (repaired)"
	return res
}

// maxCacheDepth bounds the walk: CMakeCXXCompiler.cmake sits a few levels down.
const maxCacheDepth = 6

// staleBuilds lists the platforms under build whose recorded compiler is missing.
func staleBuilds(build string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	err := filepath.WalkDir(build, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == build && errors.Is(err, fs.ErrNotExist) {
				return filepath.SkipDir // no build yet
			}
			return err
		}
		rel, _ := filepath.Rel(build, p)
		depth := len(strings.Split(rel, string(filepath.Separator)))
		if d.IsDir() && depth > maxCacheDepth {
			return filepath.SkipDir
		}
		if d.IsDir() || d.Name() != "CMakeCXXCompiler.cmake" {
			return nil
		}
		platform := strings.Split(rel, string(filepath.Separator))[0]
		if !seen[platform] && compilerMissing(p) {
			seen[platform] = true
			out = append(out, platform)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", build, err)
	}
	return out, nil
}

func compilerMissing(cacheFile string) bool {
	b, err := os.ReadFile(cacheFile) //nolint:gosec // G304: a CMake file under desktop/build
	if err != nil {
		return false
	}
	m := compilerLine.FindSubmatch(b)
	if m == nil {
		return false
	}
	compiler := filepath.FromSlash(string(m[1]))
	if !filepath.IsAbs(compiler) {
		return false
	}
	_, err = os.Stat(compiler) //nolint:gosec // G703: only checks whether the recorded compiler exists
	return errors.Is(err, fs.ErrNotExist)
}
