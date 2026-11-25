package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// fakeTracker is a GitHub-like API numbering the issues it creates.
type fakeTracker struct {
	mu     sync.Mutex
	titles []string
	auth   string
}

func (f *fakeTracker) serve(t *testing.T) string {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title string `json:"title"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.titles = append(f.titles, body.Title)
		f.auth = r.Header.Get("Authorization")
		n := len(f.titles)
		f.mu.Unlock()
		w.WriteHeader(201)
		id := string(rune('0' + n))
		_, _ = io.WriteString(w, `{"number":`+id+`,"html_url":"https://gh.test/i/`+id+`"}`)
	}))
	t.Cleanup(ts.Close)
	return ts.URL
}

func deliverSvc(t *testing.T) (*Service, *fakeTracker) {
	t.Helper()
	s := newSvc(t, analysis)
	s.Secrets = secrets.Chain{secrets.Map{"TESSIVO_LITELLM_KEY": "k", "GH_TOKEN": "gh-SECRET"}}
	f := &fakeTracker{}
	if _, err := s.AddIntegration(config.Integration{
		Name: "gh", Kind: config.IntegrationGitHub, BaseURL: f.serve(t),
		Project: "acme/shop", TokenRef: "GH_TOKEN",
	}); err != nil {
		t.Fatal(err)
	}
	return s, f
}

func TestPushIssuesRecordsAndSkips(t *testing.T) {
	s, f := deliverSvc(t)
	nb, out := analyzeOutput(t, s)
	ctx := context.Background()
	res, err := s.PushIssues(ctx, nb, out, "gh", nil, false)
	if err != nil || len(res.Sent) != 2 || len(res.Skipped) != 0 || res.Sent[0].URL != "https://gh.test/i/1" {
		t.Fatalf("push all: %v %+v", err, res)
	}
	if f.auth != "Bearer gh-SECRET" {
		t.Fatalf("auth: %q", f.auth)
	}
	res, err = s.PushIssues(ctx, nb, out, "gh", nil, false)
	if err != nil || len(res.Sent) != 0 || len(res.Skipped) != 2 {
		t.Fatalf("a second push skips what was sent: %v %+v", err, res)
	}
	res, err = s.PushIssues(ctx, nb, out, "gh", []int{1}, true)
	if err != nil || len(res.Sent) != 1 || res.Sent[0].Finding != 1 {
		t.Fatalf("again sends the picked one: %v %+v", err, res)
	}
	if _, err := s.PushIssues(ctx, nb, out, "gh", []int{7}, false); !IsInvalid(err) {
		t.Fatalf("an unknown finding: %v", err)
	}
	ds, err := s.Deliveries(nb, out)
	if err != nil || len(ds) != 3 || ds[0].Integration != "gh" || ds[0].Kind != config.IntegrationGitHub {
		t.Fatalf("deliveries: %v %+v", err, ds)
	}
	audit, _ := os.ReadFile(s.AuditPath(mustOpen(t, s, nb)))
	if !strings.Contains(string(audit), `"output.sent"`) || strings.Contains(string(audit), "gh-SECRET") {
		t.Fatalf("audited without the token:\n%s", audit)
	}
	if len(f.titles) != 3 {
		t.Fatalf("tracker got %d issues", len(f.titles))
	}
}

func TestPushNeedsATokenAndATracker(t *testing.T) {
	s, _ := deliverSvc(t)
	nb, out := analyzeOutput(t, s)
	_, _ = s.AddIntegration(config.Integration{
		Name: "jira", Kind: config.IntegrationJira, BaseURL: "https://j.test",
		Project: "OPS", TokenRef: "NO_SUCH_TOKEN",
	})
	if _, err := s.PushIssues(context.Background(), nb, out, "jira", nil, false); !IsInvalid(err) ||
		!strings.Contains(err.Error(), "tess secret set NO_SUCH_TOKEN") {
		t.Fatalf("missing token: %v", err)
	}
	_, _ = s.AddIntegration(config.Integration{Name: "share", Kind: config.IntegrationFolder, Path: t.TempDir()})
	if _, err := s.PushIssues(context.Background(), nb, out, "share", nil, false); !IsInvalid(err) {
		t.Fatalf("a folder takes no issues: %v", err)
	}
	if _, err := s.Publish(context.Background(), nb, out, "gh"); !IsInvalid(err) {
		t.Fatalf("a tracker takes no pages: %v", err)
	}
}

func TestPublishARevisionReplacesItsPage(t *testing.T) {
	s, _ := deliverSvc(t)
	dir := t.TempDir()
	_, _ = s.AddIntegration(config.Integration{Name: "share", Kind: config.IntegrationFolder, Path: dir})
	nb, out := analyzeOutput(t, s)
	d, err := s.Publish(context.Background(), nb, out, "share")
	if err != nil || d.Finding != -1 || !strings.HasSuffix(d.ID, ".md") {
		t.Fatalf("publish: %v %+v", err, d)
	}
	edited := strings.Replace(analysis, "Retries can double charge.", "Retries can double charge twice.", 1)
	v2, err := s.EditOutput(nb, out, json.RawMessage(edited))
	if err != nil {
		t.Fatal(err)
	}
	d2, err := s.Publish(context.Background(), nb, v2.ID, "share")
	if err != nil || d2.ID != d.ID {
		t.Fatalf("a revision replaces the page its parent published: %v %+v", err, d2)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("one file: %d", len(entries))
	}
	if err := s.TestIntegration(context.Background(), "share"); err != nil {
		t.Fatal(err)
	}
	if err := s.TestIntegration(context.Background(), "nope"); err == nil {
		t.Fatal("unknown integration")
	}
}

func TestArchivesLeaveDeliveryRecordsBehind(t *testing.T) {
	s, _ := deliverSvc(t)
	nb, out := analyzeOutput(t, s)
	if _, err := s.PushIssues(context.Background(), nb, out, "gh", nil, false); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.ExportNotebook(nb, true, &buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	outputs := 0
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, deliveriesFile) {
			t.Fatalf("delivery records point into the sender's trackers: %s", f.Name)
		}
		if strings.Contains(f.Name, "outputs/") {
			outputs++
		}
	}
	if outputs == 0 {
		t.Fatal("the outputs are still in the archive")
	}
}

func TestPushIsBounded(t *testing.T) {
	many := make([]Issue, MaxPushIssues+1)
	if _, err := pickIssues(many, nil); !IsInvalid(err) {
		t.Fatalf("more than %d issues at once: %v", MaxPushIssues, err)
	}
	if got, err := pickIssues(many, []int{0, 1}); err != nil || len(got) != 2 {
		t.Fatalf("a pick within the bound: %v %d", err, len(got))
	}
}
