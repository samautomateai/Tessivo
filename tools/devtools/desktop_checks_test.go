package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckSymlinks(t *testing.T) {
	a, _ := newApp(t, &fake{})
	old := makeSymlink
	t.Cleanup(func() { makeSymlink = old })
	makeSymlink = func(string, string) error { return nil }
	if r := a.checkSymlinks(); r.st != ok {
		t.Fatalf("%+v", r)
	}
	makeSymlink = func(string, string) error { return errors.New("a required privilege is not held by the client") }
	a.goos = "windows"
	if r := a.checkSymlinks(); r.st != warn || !strings.Contains(r.hint, "Developer Mode") {
		t.Fatalf("windows: %+v", r)
	}
	a.goos = "linux"
	if r := a.checkSymlinks(); r.st != warn || strings.Contains(r.hint, "Developer Mode") {
		t.Fatalf("other systems get a general hint: %+v", r)
	}
}

// cmakeCache writes a CMakeCXXCompiler.cmake for platform naming compiler.
func cmakeCache(t *testing.T, root, platform, compiler string) string {
	t.Helper()
	dir := filepath.Join(root, "desktop", "build", platform, "x64", "CMakeFiles", "4.1.0")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	text := `set(CMAKE_CXX_COMPILER "` + filepath.ToSlash(compiler) + `")` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "CMakeCXXCompiler.cmake"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "desktop", "build", platform)
}

func TestCheckBuildCache(t *testing.T) {
	a, _ := newApp(t, &fake{})
	if r := a.checkBuildCache(true); r.st != ok {
		t.Fatalf("no build folder yet: %+v", r)
	}
	present := filepath.Join(a.root, "cl.exe")
	if err := os.WriteFile(present, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	good := cmakeCache(t, a.root, "linux", present)
	gone := cmakeCache(t, a.root, "windows", filepath.Join(a.root, "MSVC", "14.51", "cl.exe"))
	if r := a.checkBuildCache(false); r.st != warn || !strings.Contains(r.detail, "windows") ||
		strings.Contains(r.detail, "linux") {
		t.Fatalf("report only: %+v", r)
	}
	if _, err := os.Stat(gone); err != nil {
		t.Fatal("-check must not delete anything")
	}
	if r := a.checkBuildCache(true); r.st != ok || !strings.Contains(r.detail, "repaired") {
		t.Fatalf("fix: %+v", r)
	}
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Fatal("the stale build folder must be deleted")
	}
	if _, err := os.Stat(good); err != nil {
		t.Fatal("a build with its compiler present must be kept")
	}
}
