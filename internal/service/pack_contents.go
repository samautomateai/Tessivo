package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/pack"
	"github.com/samautomateai/tessivo/internal/template"
	"github.com/samautomateai/tessivo/internal/worktype"
)

const auditMCPTrust = "mcp.trust"

// loadedPack is an added pack that loaded.
type loadedPack struct {
	src config.PackSource
	p   pack.Pack
}

// loadedPacks loads every added pack; one that fails to load is left out
// (Packs reports why).
func (s *Service) loadedPacks() []loadedPack {
	var out []loadedPack
	for _, src := range s.cfg().Packs {
		if p, err := pack.Load(s.packDir(src)); err == nil {
			out = append(out, loadedPack{src: src, p: p})
		}
	}
	return out
}

// packRef names an item of a pack as the app shows it: "<pack>/<item>".
func packRef(packID, item string) string { return packID + "/" + item }

// markRank sorts required items first, then recommended, then the rest.
func markRank(mark string) int {
	switch mark {
	case pack.MarkRequired:
		return 0
	case pack.MarkRecommended:
		return 1
	}
	return 2
}

// packTemplates are the pack templates, IDs as "<pack>/<id>".
func (s *Service) packTemplates(workType string) []TemplateInfo {
	var out []TemplateInfo
	for _, lp := range s.loadedPacks() {
		for _, t := range lp.p.Templates {
			if workType != "" && t.WorkType != workType {
				continue
			}
			mark := lp.p.Mark("template:" + t.ID)
			t.ID = packRef(lp.src.ID, t.ID)
			out = append(out, TemplateInfo{Template: t, Pack: lp.src.ID, Mark: mark})
		}
	}
	return out
}

// findTemplate resolves a template ID, including "<pack>/<id>".
func (s *Service) findTemplate(id string) (template.Template, string, string, error) {
	packID, item, ok := strings.Cut(id, "/")
	if !ok {
		t, err := s.Templates.Get(id)
		return t, "", "", err
	}
	for _, lp := range s.loadedPacks() {
		if lp.src.ID != packID {
			continue
		}
		for _, t := range lp.p.Templates {
			if t.ID == item {
				return t, packID, lp.p.Mark("template:" + item), nil
			}
		}
	}
	return template.Template{}, "", "", fmt.Errorf("template %s: %w", id, template.ErrNotFound)
}

// LibraryPromptInfo is a starter or pack prompt for the prompt library.
type LibraryPromptInfo struct {
	worktype.LibraryPrompt
	Pack string `json:"pack,omitempty"`
	Mark string `json:"mark,omitempty"`
}

// PromptLibrary lists the pack prompts (required, then recommended, then the
// rest) and then the starter prompts, optionally for one work type; a pack
// prompt without a work type fits every work type.
func (s *Service) PromptLibrary(workType string) []LibraryPromptInfo {
	var out []LibraryPromptInfo
	for _, lp := range s.loadedPacks() {
		for _, p := range lp.p.Prompts {
			if workType != "" && p.WorkType != "" && p.WorkType != workType {
				continue
			}
			out = append(out, LibraryPromptInfo{
				LibraryPrompt: worktype.LibraryPrompt{
					ID: packRef(lp.src.ID, p.Name), Name: p.Name, WorkType: p.WorkType,
					Text: p.Text,
				},
				Pack: lp.src.ID, Mark: lp.p.Mark("prompt:" + p.Name),
			})
		}
	}
	slices.SortStableFunc(out, func(a, b LibraryPromptInfo) int { return markRank(a.Mark) - markRank(b.Mark) })
	for _, p := range worktype.LibraryFor(workType) {
		out = append(out, LibraryPromptInfo{LibraryPrompt: p})
	}
	return out
}

// entryHash identifies a pack server entry; trust is given to this hash, so a
// changed command, URL, argument, header or environment value needs trust again.
func entryHash(srv config.MCPServer) string {
	b, _ := json.Marshal(srv) // map keys are sorted, so the encoding is stable
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// packServer is an MCP server from a pack, named "<pack>/<server>".
type packServer struct {
	srv     config.MCPServer
	pack    string
	mark    string
	hash    string
	trusted bool
}

func (s *Service) packServers() []packServer {
	trusted := s.cfg().MCP.Trusted
	var out []packServer
	for _, lp := range s.loadedPacks() {
		for _, srv := range lp.p.MCP {
			h := entryHash(srv)
			mark := lp.p.Mark("mcp:" + srv.Name)
			srv.Name = packRef(lp.src.ID, srv.Name)
			ps := packServer{srv: srv, pack: lp.src.ID, mark: mark, hash: h, trusted: trusted[srv.Name] == h}
			out = append(out, ps)
		}
	}
	return out
}

// mcpMissing explains why name cannot be used: an untrusted pack server, or
// not configured at all.
func (s *Service) mcpMissing(name string) error {
	for _, ps := range s.packServers() {
		if ps.srv.Name == name {
			return invalidf("mcp server %q comes from a pack and is not trusted; trust it in Settings first", name)
		}
	}
	return invalidf("mcp server %q is not configured; add it first", name)
}

// TrustMCP trusts or stops trusting a pack server ("<pack>/<server>"). Trust
// is recorded against the server's current entry.
func (s *Service) TrustMCP(name string, trust bool) error {
	var found *packServer
	for _, ps := range s.packServers() {
		if ps.srv.Name == name {
			found = &ps
		}
	}
	if found == nil {
		return invalidf("%q is not an MCP server from a pack", name)
	}
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	next := map[string]string{}
	for k, v := range s.Config.MCP.Trusted {
		next[k] = v
	}
	if trust {
		next[name] = found.hash
	} else {
		delete(next, name)
	}
	if err := s.saveConfigLocked(func(f *config.File) { f.MCP.Trusted = next }); err != nil {
		return err
	}
	s.Config.MCP.Trusted = next
	if s.UserAuditPath != "" {
		if l, err := audit.Open(s.UserAuditPath, s.actor()); err == nil {
			_ = l.Append(auditMCPTrust, name, map[string]any{"trust": trust, "entry_sha256": found.hash})
			_ = l.Close()
		}
	}
	return nil
}
