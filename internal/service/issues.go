package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/worktype"
)

// Issue is a tracker-ready issue built from an Analyze finding (WT-5).
type Issue struct {
	Index    int      `json:"index"` // the finding's position; 0 for a whole legacy output
	Title    string   `json:"title"`
	Severity string   `json:"severity"`
	Body     string   `json:"body"` // Markdown
	Labels   []string `json:"labels"`
}

// Bounds on what an issue carries; trackers refuse much longer titles.
const (
	maxIssueTitle = 250
	maxIssueLabel = 50
	maxIssueBody  = 60000
)

type analyzeData struct {
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	RiskLevel string     `json:"risk_level"`
	Findings  *[]finding `json:"findings"`
	Impacted  []string   `json:"impacted_areas"`
	Evidence  []struct {
		Claim  string `json:"claim"`
		Source string `json:"source"`
	} `json:"evidence"`
	Mitigations []string `json:"mitigations"`
}

type finding struct {
	Title    string   `json:"title"`
	Severity string   `json:"severity"`
	Detail   string   `json:"detail"`
	Steps    []string `json:"steps"`
	Labels   []string `json:"labels"`
}

// Issues turns an Analyze output's findings into issues. An output saved
// before findings existed becomes one issue made from the whole output.
func (s *Service) Issues(notebookID, outputID string) ([]Issue, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return nil, err
	}
	if ref.out.WorkType != worktype.Analyze {
		return nil, invalidf("only Analyze outputs become issues; this one is %s", ref.out.WorkType)
	}
	var d analyzeData
	if err := json.Unmarshal(ref.out.Data, &d); err != nil {
		return nil, fmt.Errorf("issues: %w", err)
	}
	footer := fmt.Sprintf("\n\n---\nFrom Tessivo: notebook %q, output %q (%s).", ref.nb.Manifest.Name, d.Title,
		ref.out.ID)
	if d.Findings == nil {
		body := d.Summary + evidenceMD(d) + stepsMD("Suggested steps", d.Mitigations) + footer
		return []Issue{newIssue(0, d.Title, d.RiskLevel, body, d.Impacted)}, nil
	}
	out := make([]Issue, 0, len(*d.Findings))
	for i, f := range *d.Findings {
		body := f.Detail + stepsMD("Steps", f.Steps) + evidenceMD(d) + footer
		out = append(out, newIssue(i, f.Title, f.Severity, body, f.Labels))
	}
	return out, nil
}

func newIssue(i int, title, severity, body string, labels []string) Issue {
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		title = "Finding " + fmt.Sprint(i+1)
	}
	var ls []string
	for _, l := range labels {
		if l = strings.TrimSpace(l); l != "" {
			ls = append(ls, cut(l, maxIssueLabel))
		}
	}
	if severity != "" {
		ls = append(ls, "severity:"+severity)
	}
	if ls == nil {
		ls = []string{}
	}
	return Issue{
		Index: i, Title: cut(title, maxIssueTitle), Severity: severity, Body: cut(body, maxIssueBody),
		Labels: ls,
	}
}

func stepsMD(heading string, steps []string) string {
	if len(steps) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## " + heading + "\n\n")
	for i, st := range steps {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, st)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func evidenceMD(d analyzeData) string {
	if len(d.Evidence) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## Evidence\n\n")
	for _, e := range d.Evidence {
		fmt.Fprintf(&sb, "- %s (%s)\n", e.Claim, e.Source)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// cut shortens s to at most n runes.
func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// withFindings adds an empty findings list to an Analyze output saved before
// findings existed, so it still fits the schema when edited.
func withFindings(data []byte) []byte {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return data
	}
	if _, ok := m["findings"]; ok {
		return data
	}
	m["findings"] = json.RawMessage("[]")
	b, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return b
}
