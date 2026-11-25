package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/notebook"
)

const analysis = `{"title":"Payment retry review","summary":"Retries can double charge.","risk_score":7,` +
	`"risk_level":"high","findings":[{"title":"Retries are not idempotent","severity":"high",` +
	`"detail":"A timeout retries the charge.","steps":["Time out the gateway","Watch two charges"],` +
	`"labels":["payments","bug"]},{"title":"No alert on double charge","severity":"medium","detail":"Nobody ` +
	`notices.","steps":[],"labels":[]}],"impacted_areas":["checkout"],"evidence":[{"claim":"retry loop",` +
	`"source":"pay.go:42"}],"mitigations":["Add an idempotency key"],"open_questions":[]}`

// legacyAnalysis is an Analyze output saved before findings existed.
const legacyAnalysis = `{"title":"Old review","summary":"Old summary.","risk_score":4,"risk_level":"medium",` +
	`"impacted_areas":["auth","api"],"evidence":[{"claim":"c","source":"a.go"}],` +
	`"mitigations":["Rotate keys","Add tests"],"open_questions":[]}`

func analyzeOutput(t *testing.T, s *Service) (string, string) {
	t.Helper()
	info, err := s.CreateNotebook("Pay", "analyze")
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Run(context.Background(), RunRequest{Notebook: mustOpen(t, s, info.ID), Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return info.ID, out.OutputID
}

func TestIssuesFromFindings(t *testing.T) {
	s := newSvc(t, analysis)
	nb, out := analyzeOutput(t, s)
	issues, err := s.Issues(nb, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("one issue per finding: %+v", issues)
	}
	first := issues[0]
	if first.Index != 0 || first.Title != "Retries are not idempotent" || first.Severity != "high" {
		t.Fatalf("%+v", first)
	}
	for _, want := range []string{"A timeout retries the charge.", "1. Time out the gateway", "pay.go:42", "Pay"} {
		if !strings.Contains(first.Body, want) {
			t.Errorf("body lacks %q:\n%s", want, first.Body)
		}
	}
	if strings.Join(first.Labels, ",") != "payments,bug,severity:high" {
		t.Fatalf("labels: %v", first.Labels)
	}
	if issues[1].Labels[0] != "severity:medium" {
		t.Fatalf("labels: %v", issues[1].Labels)
	}
}

func TestIssuesFromALegacyOutputAndOtherWorkTypes(t *testing.T) {
	s := newSvc(t, analysis)
	nbID, first := analyzeOutput(t, s)
	nb := mustOpen(t, s, nbID)
	made, _, _ := nb.GetOutput(first)
	old := notebook.Output{
		ID: notebook.NewID(), RunID: made.RunID, WorkType: "analyze", Title: "Old review",
		CreatedAt: nowRFC3339(), Data: json.RawMessage(legacyAnalysis),
	}
	if err := nb.SaveOutput(old, "# Old review"); err != nil {
		t.Fatal(err)
	}
	issues, err := s.Issues(nbID, old.ID)
	if err != nil || len(issues) != 1 {
		t.Fatalf("a legacy output is one issue: %v %+v", err, issues)
	}
	is := issues[0]
	if is.Title != "Old review" || is.Severity != "medium" || !strings.Contains(is.Body, "1. Rotate keys") ||
		strings.Join(is.Labels, ",") != "auth,api,severity:medium" {
		t.Fatalf("%+v", is)
	}
	edited := strings.Replace(legacyAnalysis, "Old summary.", "New summary.", 1)
	if _, err := s.EditOutput(nbID, old.ID, json.RawMessage(edited)); err != nil {
		t.Fatalf("a legacy output still edits: %v", err)
	}

	s2 := newSvc(t, summary)
	info, _ := s2.CreateNotebook("Board", "summarize")
	o, err := s2.Run(context.Background(), RunRequest{Notebook: mustOpen(t, s2, info.ID), Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Issues(info.ID, o.OutputID); !IsInvalid(err) {
		t.Fatalf("only Analyze outputs become issues: %v", err)
	}
}
