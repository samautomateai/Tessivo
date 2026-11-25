// Package pack loads team packs (PR-5, TPL-7, MCP-5): a folder of templates,
// library prompts and MCP server entries with a tessivo-pack.json manifest
// that marks items recommended or required. Packs come from other people, so
// every file is size-bounded, validated with the same rules as the user's own
// items, and checked for secrets; a bad item is skipped and reported while
// the rest of the pack still loads. Nothing in a pack runs by itself.
package pack

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/template"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// The pack layout.
const (
	FormatVersion = 1
	ManifestFile  = "tessivo-pack.json"
	TemplatesDir  = "templates"
	PromptsDir    = "prompts"
	MCPFile       = "mcp-servers.json"
	ReadmeFile    = "README.md"

	MarkRequired    = "required"
	MarkRecommended = "recommended"
)

// Limits (spec: organize and share).
const (
	MaxTemplates = 200
	MaxPrompts   = 500
	MaxServers   = 50
	MaxBytes     = 4 << 20
	MaxPacks     = 20
	maxManifest  = 64 << 10
)

// The limits Load uses; tests lower them.
var (
	maxTemplates       = MaxTemplates
	maxBytes     int64 = MaxBytes
)

var (
	idPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	refPattern = regexp.MustCompile(`^(template|prompt|mcp):[A-Za-z0-9._-]{1,64}$`)
)

// Manifest is tessivo-pack.json.
type Manifest struct {
	FormatVersion int      `json:"formatVersion"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Publisher     string   `json:"publisher,omitempty"`
	Version       string   `json:"version,omitempty"`
	Recommended   []string `json:"recommended,omitempty"` // item refs: template:<id>, prompt:<name>, mcp:<name>
	Required      []string `json:"required,omitempty"`
}

// Prompt is a library prompt from a pack.
type Prompt struct {
	Name     string             `json:"name"`
	Text     string             `json:"text"`
	WorkType string             `json:"workType,omitempty"`
	Settings config.RunSettings `json:"settings,omitzero"`
}

// Pack is a loaded pack.
type Pack struct {
	Manifest  Manifest
	Templates []template.Template
	Prompts   []Prompt
	MCP       []config.MCPServer
	Problems  []string // "<file>: <reason>" for each item that was skipped
	README    string
}

// Mark says whether the pack marks ref required, recommended or neither ("").
func (p Pack) Mark(ref string) string {
	switch {
	case slices.Contains(p.Manifest.Required, ref):
		return MarkRequired
	case slices.Contains(p.Manifest.Recommended, ref):
		return MarkRecommended
	}
	return ""
}

// Load reads the pack in dir. It fails only when the manifest is missing or
// invalid or the pack is larger than MaxBytes; bad items go to Problems.
func Load(dir string) (Pack, error) {
	m, err := readManifest(dir)
	if err != nil {
		return Pack{}, err
	}
	if err := checkSize(dir); err != nil {
		return Pack{}, err
	}
	p := Pack{Manifest: m}
	p.loadTemplates(dir)
	p.loadPrompts(dir)
	p.loadMCP(dir)
	if b, err := readLimited(filepath.Join(dir, ReadmeFile), maxManifest); err == nil {
		p.README = string(b)
	}
	return p, nil
}

func readManifest(dir string) (Manifest, error) {
	b, err := readLimited(filepath.Join(dir, ManifestFile), maxManifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("pack: %s: %w", ManifestFile, err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("pack: %s: %w", ManifestFile, err)
	}
	return m, m.Check()
}

// Check validates the manifest.
func (m Manifest) Check() error {
	var errs []error
	if m.FormatVersion != FormatVersion {
		errs = append(errs, fmt.Errorf("format version %d (this Tessivo reads %d)", m.FormatVersion, FormatVersion))
	}
	if !idPattern.MatchString(m.ID) {
		errs = append(errs, fmt.Errorf("id %q: use lowercase letters, digits and '-'", m.ID))
	}
	if strings.TrimSpace(m.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}
	for _, ref := range slices.Concat(m.Recommended, m.Required) {
		if !refPattern.MatchString(ref) {
			errs = append(errs, fmt.Errorf("item %q: want template:<id>, prompt:<name> or mcp:<name>", ref))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("pack %s: %w", ManifestFile, err)
	}
	return nil
}

// checkSize refuses a pack whose regular files (outside .git) exceed maxBytes.
func checkSize(dir string) error {
	var total int64
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return fmt.Errorf("pack: %w", err)
			}
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	if total > maxBytes {
		return fmt.Errorf("pack is larger than %d KB", maxBytes>>10)
	}
	return nil
}

// jsonFiles lists the regular *.json files of dir/sub, sorted; links are skipped.
func jsonFiles(dir, sub string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, sub))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, sub+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func (p *Pack) problem(file string, err error) {
	p.Problems = append(p.Problems, file+": "+err.Error())
}

func (p *Pack) loadTemplates(dir string) {
	for _, rel := range jsonFiles(dir, TemplatesDir) {
		if len(p.Templates) >= maxTemplates {
			p.Problems = append(p.Problems, fmt.Sprintf("%s: at most %d templates; the rest are skipped",
				TemplatesDir, maxTemplates))
			return
		}
		var t template.Template
		if err := readJSON(dir, rel, template.MaxFileBytes, &t); err != nil {
			p.problem(rel, err)
			continue
		}
		t.Source = template.SourcePack
		if err := t.Validate(); err != nil {
			p.problem(rel, err)
			continue
		}
		p.Templates = append(p.Templates, t)
	}
}

func (p *Pack) loadPrompts(dir string) {
	for _, rel := range jsonFiles(dir, PromptsDir) {
		if len(p.Prompts) >= MaxPrompts {
			p.Problems = append(p.Problems, fmt.Sprintf("%s: at most %d prompts; the rest are skipped", PromptsDir,
				MaxPrompts))
			return
		}
		var pr Prompt
		if err := readJSON(dir, rel, template.MaxFileBytes, &pr); err != nil {
			p.problem(rel, err)
			continue
		}
		if err := pr.Check(); err != nil {
			p.problem(rel, err)
			continue
		}
		p.Prompts = append(p.Prompts, pr)
	}
}

// Check validates a pack prompt like a saved prompt, plus its work type and
// that it holds no secret.
func (pr Prompt) Check() error {
	if err := (notebook.Prompt{Name: pr.Name, Text: pr.Text, Settings: pr.Settings}).Check(); err != nil {
		return fmt.Errorf("%w", err)
	}
	if pr.WorkType != "" {
		if _, err := worktype.Get(pr.WorkType); err != nil {
			return fmt.Errorf("prompt %s: %w", pr.Name, err)
		}
	}
	if secrets.RedactPatterns(pr.Text) != pr.Text {
		return fmt.Errorf("prompt %s looks like it contains a secret", pr.Name)
	}
	return nil
}

func (p *Pack) loadMCP(dir string) {
	var servers []config.MCPServer
	if err := readJSON(dir, MCPFile, maxManifest, &servers); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			p.problem(MCPFile, err)
		}
		return
	}
	seen := map[string]bool{}
	for _, s := range servers {
		switch err := s.Check(); {
		case len(p.MCP) >= MaxServers:
			p.problem(MCPFile, fmt.Errorf("at most %d servers; the rest are skipped", MaxServers))
			return
		case err != nil:
			p.problem(MCPFile, err)
		case seen[s.Name]:
			p.problem(MCPFile, fmt.Errorf("mcp server %q: duplicate name", s.Name))
		default:
			seen[s.Name] = true
			p.MCP = append(p.MCP, s)
		}
	}
}

func readJSON(dir, rel string, limit int64, v any) error {
	b, err := readLimited(filepath.Join(dir, filepath.FromSlash(rel)), limit)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("not valid JSON: %w", err)
	}
	return nil
}

// readLimited reads a regular file of at most limit bytes (links refused).
func readLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("larger than %d KB", limit>>10)
	}
	b, err := os.ReadFile(path) //nolint:gosec // G304: a file inside the pack being loaded
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return b, nil
}
