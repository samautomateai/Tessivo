package service

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/pack"
	"github.com/samautomateai/tessivo/internal/template"
)

// svcWithTeamPack adds the team pack (see teamPackDir) as a folder pack.
func svcWithTeamPack(t *testing.T) (*Service, string) {
	t.Helper()
	s := packSvc(t)
	dir := teamPackDir(t, "team")
	if _, err := s.AddPack(context.Background(), dir, ""); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func TestPackTemplatesAreListedAndUsable(t *testing.T) {
	s, _ := svcWithTeamPack(t)
	s.Templates = template.Library{Dir: t.TempDir()}
	list, err := s.ListTemplates("")
	if err != nil {
		t.Fatal(err)
	}
	if list[0].ID != "team/risk-review" || list[0].Pack != "team" || list[0].Mark != pack.MarkRequired {
		t.Fatalf("required pack template first: %+v", list[0])
	}
	if got, err := s.GetTemplate("team/risk-review"); err != nil || got.Name != "Risk review" {
		t.Fatalf("get: %v %+v", err, got)
	}
	nb, err := s.CreateFromTemplate("team/risk-review", "Release review")
	if err != nil || nb.Name != "Release review" {
		t.Fatalf("create: %v %+v", err, nb)
	}
	if only, _ := s.ListTemplates("summarize"); len(only) > 0 && only[0].Pack == "team" {
		t.Fatal("the work type filter must apply to pack templates")
	}
}

func TestPackPromptsJoinTheLibrary(t *testing.T) {
	s, _ := svcWithTeamPack(t)
	lib := s.PromptLibrary("")
	var found bool
	for _, p := range lib {
		if p.ID == "team/triage" && p.Pack == "team" && p.Text == "Triage." {
			found = true
		}
	}
	if !found || len(lib) <= 1 {
		t.Fatalf("library: %+v", lib)
	}
}

func TestPackServersNeedTrust(t *testing.T) {
	s, dir := svcWithTeamPack(t)
	var listed *MCPServerInfo
	for _, srv := range s.MCPServers() {
		if srv.Name == "team/github" {
			listed = &srv
		}
	}
	if listed == nil || listed.Trusted || listed.Pack != "team" || listed.Mark != pack.MarkRequired {
		t.Fatalf("listed: %+v", listed)
	}
	info, _ := s.CreateNotebook("Ops", "analyze")
	if _, err := s.SetNotebookMCP(info.ID, []string{"team/github"}); err == nil ||
		!strings.Contains(err.Error(), "trust") {
		t.Fatalf("an untrusted pack server must be refused with a hint: %v", err)
	}
	if err := s.TrustMCP("team/github", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetNotebookMCP(info.ID, []string{"team/github"}); err != nil {
		t.Fatalf("after trust: %v", err)
	}
	f, _ := config.Load(s.ConfigPath)
	if f.MCP.Trusted["team/github"] == "" {
		t.Fatal("trust must be saved")
	}
	// The pack changes the server's command: the old trust no longer matches.
	servers := []config.MCPServer{{Name: "github", Command: "evil-binary"}}
	m := pack.Manifest{ID: "team", Name: "Team pack"}
	if err := pack.Export(dir, m, nil, nil, servers); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.mcpConfig().Find("team/github"); ok {
		t.Fatal("a changed pack server must need trust again")
	}
	if err := s.TrustMCP("team/nope", true); !IsInvalid(err) && err == nil {
		t.Fatal("trusting an unknown server must fail")
	}
	if err := s.TrustMCP("team/github", false); err != nil {
		t.Fatal(err)
	}
	log, _ := os.ReadFile(s.UserAuditPath)
	if strings.Count(string(log), auditMCPTrust) < 2 {
		t.Fatal("trust decisions must be audited")
	}
}

func TestOwnServersStayTrusted(t *testing.T) {
	s := packSvc(t)
	if _, err := s.AddMCPServer(config.MCPServer{Name: "docs", URL: "https://docs.example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	for _, srv := range s.MCPServers() {
		if srv.Name == "docs" && (!srv.Trusted || srv.Pack != "") {
			t.Fatalf("%+v", srv)
		}
	}
	if err := s.TrustMCP("docs", true); err == nil {
		t.Fatal("trust applies to pack servers only")
	}
}
