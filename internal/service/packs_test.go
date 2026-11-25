package service

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/pack"
	"github.com/samautomateai/tessivo/internal/template"
)

// packSvc is a service with a config file, so packs have a folder.
func packSvc(t *testing.T) *Service {
	t.Helper()
	s := newSvc(t, summary)
	s.ConfigPath = filepath.Join(t.TempDir(), "config.json")
	return s
}

// teamPackDir writes a pack with one template, one prompt and one server.
func teamPackDir(t *testing.T, id string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), id)
	tpl := template.Template{
		ID: "risk-review", Name: "Risk review", WorkType: "analyze",
		Prompts: []template.Prompt{{Name: "review", Text: "Assess the risk."}},
	}
	m := pack.Manifest{ID: id, Name: "Team pack", Required: []string{"template:risk-review", "mcp:github"}}
	servers := []config.MCPServer{{Name: "github", Command: "npx", Args: []string{"-y", "server-github"}}}
	if err := pack.Export(dir, m, []template.Template{tpl}, []pack.Prompt{{Name: "triage", Text: "Triage."}},
		servers); err != nil {
		t.Fatal(err)
	}
	return dir
}

func zipOfDir(t *testing.T, dir string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "pack.zip")
	f, _ := os.Create(file)
	zw := zip.NewWriter(f)
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, _ error) error {
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, p)
			w, _ := zw.Create(filepath.ToSlash(rel))
			b, _ := os.ReadFile(p)
			_, _ = w.Write(b)
		}
		return nil
	})
	_ = zw.Close()
	_ = f.Close()
	return file
}

func TestAddFolderAndZipPacks(t *testing.T) {
	s := packSvc(t)
	ctx := context.Background()
	folder := teamPackDir(t, "team")
	info, err := s.AddPack(ctx, folder, "")
	if err != nil || info.ID != "team" || info.Kind != pack.KindFolder || info.Templates != 1 || info.Servers != 1 {
		t.Fatalf("%v %+v", err, info)
	}
	if _, err := s.AddPack(ctx, folder, ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("adding the same pack twice: %v", err)
	}
	z, err := s.AddPack(ctx, zipOfDir(t, teamPackDir(t, "ops")), "")
	if err != nil || z.Kind != pack.KindZip {
		t.Fatalf("%v %+v", err, z)
	}
	copyDir := filepath.Join(filepath.Dir(s.ConfigPath), "packs", "ops")
	if _, err := os.Stat(filepath.Join(copyDir, pack.ManifestFile)); err != nil {
		t.Fatalf("zip pack not unpacked: %v", err)
	}
	f, _ := config.Load(s.ConfigPath)
	if len(f.Packs) != 2 || f.Packs[0].ID != "team" {
		t.Fatalf("config packs: %+v", f.Packs)
	}
	if list := s.Packs(); len(list) != 2 {
		t.Fatalf("packs: %+v", list)
	}
	log, _ := os.ReadFile(s.UserAuditPath)
	if !strings.Contains(string(log), auditPackAdd) {
		t.Fatal("pack add not audited")
	}
}

func TestPackSourcesAreChecked(t *testing.T) {
	s := packSvc(t)
	ctx := context.Background()
	for _, src := range []string{"file:///tmp/x", "ext::sh -c id", "relative/dir", t.TempDir()} {
		if _, err := s.AddPack(ctx, src, ""); !IsInvalid(err) {
			t.Errorf("%q: %v", src, err)
		}
	}
	s.Config.Packs = make([]config.PackSource, config.MaxPacks)
	if _, err := s.AddPack(ctx, teamPackDir(t, "team"), ""); !IsInvalid(err) {
		t.Fatalf("21st pack: %v", err)
	}
}

func TestRemovePackDropsItsCopyAndTrust(t *testing.T) {
	s := packSvc(t)
	ctx := context.Background()
	folder := teamPackDir(t, "team")
	if _, err := s.AddPack(ctx, folder, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPack(ctx, zipOfDir(t, teamPackDir(t, "ops")), ""); err != nil {
		t.Fatal(err)
	}
	s.Config.MCP.Trusted = map[string]string{"ops/github": "abc", "team/github": "def"}
	if err := s.RemovePack("ops"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.ConfigPath), "packs", "ops")); !os.IsNotExist(err) {
		t.Fatal("the zip copy must be deleted")
	}
	if err := s.RemovePack("team"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Fatal("a folder pack's own folder must stay")
	}
	f, _ := config.Load(s.ConfigPath)
	if len(f.Packs) != 0 || len(f.MCP.Trusted) != 0 {
		t.Fatalf("config: %+v %+v", f.Packs, f.MCP.Trusted)
	}
	if err := s.RemovePack("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRefreshKeepsTheLastGoodCopy(t *testing.T) {
	s := packSvc(t)
	dir := filepath.Join(filepath.Dir(s.ConfigPath), "packs", "team")
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(teamPackDir(t, "team"), dir); err != nil {
		t.Fatal(err)
	}
	// A git pack whose remote is gone: refresh fails, the copy stays usable.
	s.Config.Packs = []config.PackSource{{ID: "team", Source: "https://example.invalid/team.git", Kind: pack.KindGit}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // no network in tests: a cancelled refresh fails at once
	list := s.RefreshPacks(ctx)
	if len(list) != 1 || list[0].Error == "" || list[0].Templates != 1 {
		t.Fatalf("%+v", list)
	}
}

func TestRemovingAnMCPServerKeepsPackTrust(t *testing.T) {
	s := packSvc(t)
	if _, err := s.AddMCPServer(config.MCPServer{Name: "docs", URL: "https://docs.example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	s.Config.MCP.Trusted = map[string]string{"team/github": "abc"}
	if err := s.RemoveMCPServer("docs"); err != nil {
		t.Fatal(err)
	}
	if s.Config.MCP.Trusted["team/github"] != "abc" {
		t.Fatalf("trust lost: %+v", s.Config.MCP.Trusted)
	}
}
