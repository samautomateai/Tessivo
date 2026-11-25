package deliver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
)

// recorder answers with reply and keeps the last request.
type recorder struct {
	method, path, auth, body string
	header                   http.Header
}

func serve(t *testing.T, status int, reply string) (*httptest.Server, *recorder) {
	t.Helper()
	rec := &recorder{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		rec.method, rec.path, rec.auth, rec.body, rec.header = r.Method, r.URL.EscapedPath(),
			r.Header.Get("Authorization"), string(b), r.Header.Clone()
		w.WriteHeader(status)
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(ts.Close)
	return ts, rec
}

var issue = Issue{
	Title: "Retries double charge", Body: "## Steps\n\n1. a", Labels: []string{"pay ments", "severity:high"},
}

func TestGitHubIssue(t *testing.T) {
	ts, rec := serve(t, 201, `{"number":12,"html_url":"https://github.test/acme/shop/issues/12"}`)
	c, err := New(config.Integration{Kind: config.IntegrationGitHub, BaseURL: ts.URL, Project: "acme/shop"}, "tok")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := c.CreateIssue(context.Background(), issue)
	if err != nil || ref.ID != "12" || ref.URL != "https://github.test/acme/shop/issues/12" {
		t.Fatalf("%v %+v", err, ref)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(rec.body), &body)
	if rec.method != "POST" || rec.path != "/repos/acme/shop/issues" || rec.auth != "Bearer tok" ||
		body["title"] != issue.Title || body["body"] != issue.Body || len(body["labels"].([]any)) != 2 {
		t.Fatalf("%+v", rec)
	}
}

func TestGitLabIssue(t *testing.T) {
	ts, rec := serve(t, 201, `{"iid":3,"web_url":"https://gitlab.test/team/app/-/issues/3"}`)
	c, _ := New(config.Integration{Kind: config.IntegrationGitLab, BaseURL: ts.URL, Project: "team/app"}, "tok")
	ref, err := c.CreateIssue(context.Background(), issue)
	if err != nil || ref.ID != "3" || !strings.HasSuffix(ref.URL, "/issues/3") {
		t.Fatalf("%v %+v", err, ref)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(rec.body), &body)
	if rec.path != "/api/v4/projects/team%2Fapp/issues" || rec.header.Get("Private-Token") != "tok" || rec.auth != "" ||
		body["description"] != issue.Body || body["labels"] != "pay ments,severity:high" {
		t.Fatalf("%+v %v", rec, body)
	}
}

func TestJiraIssueBearerAndBasic(t *testing.T) {
	ts, rec := serve(t, 201, `{"id":"10001","key":"OPS-7"}`)
	in := config.Integration{Kind: config.IntegrationJira, BaseURL: ts.URL, Project: "OPS"}
	c, _ := New(in, "pat")
	ref, err := c.CreateIssue(context.Background(), issue)
	if err != nil || ref.ID != "OPS-7" || ref.URL != ts.URL+"/browse/OPS-7" {
		t.Fatalf("%v %+v", err, ref)
	}
	var body struct {
		Fields struct {
			Project   struct{ Key string }  `json:"project"`
			Summary   string                `json:"summary"`
			IssueType struct{ Name string } `json:"issuetype"`
			Labels    []string              `json:"labels"`
		} `json:"fields"`
	}
	_ = json.Unmarshal([]byte(rec.body), &body)
	if rec.path != "/rest/api/2/issue" || rec.auth != "Bearer pat" || body.Fields.Project.Key != "OPS" ||
		body.Fields.IssueType.Name != "Task" || strings.Join(body.Fields.Labels, ",") != "pay-ments,severity:high" {
		t.Fatalf("%+v %+v", rec, body)
	}
	in.User, in.IssueType = "me@acme.test", "Bug"
	c, _ = New(in, "api-token")
	if _, err := c.CreateIssue(context.Background(), issue); err != nil {
		t.Fatal(err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("me@acme.test:api-token"))
	if rec.auth != want || !strings.Contains(rec.body, `"name":"Bug"`) {
		t.Fatalf("basic auth: %q %s", rec.auth, rec.body)
	}
}

func TestErrorsAreBoundedAndRedacted(t *testing.T) {
	ts, _ := serve(t, 401, `{"message":"bad credentials for tok-SECRET `+strings.Repeat("x", 2000)+`"}`)
	c, _ := New(config.Integration{Kind: config.IntegrationGitHub, BaseURL: ts.URL, Project: "a/b"}, "tok-SECRET")
	_, err := c.CreateIssue(context.Background(), issue)
	if err == nil || strings.Contains(err.Error(), "tok-SECRET") || len(err.Error()) > 400 ||
		!strings.Contains(err.Error(), "401") {
		t.Fatalf("error: %v", err)
	}
	big, _ := serve(t, 200, strings.Repeat("y", MaxResponse+10))
	c, _ = New(config.Integration{Kind: config.IntegrationGitHub, BaseURL: big.URL, Project: "a/b"}, "t")
	if _, err := c.CreateIssue(context.Background(), issue); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized: %v", err)
	}
	redirect := httptest.NewServer(http.RedirectHandler("https://elsewhere.test/", http.StatusFound))
	defer redirect.Close()
	c, _ = New(config.Integration{Kind: config.IntegrationGitHub, BaseURL: redirect.URL, Project: "a/b"}, "t")
	if _, err := c.CreateIssue(context.Background(), issue); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatalf("redirects are not followed: %v", err)
	}
	if _, err := New(config.Integration{Kind: config.IntegrationNotion}, "t"); err != nil {
		t.Fatal(err)
	}
	notion, _ := New(config.Integration{Kind: config.IntegrationNotion}, "t")
	if _, err := notion.CreateIssue(context.Background(), issue); err == nil {
		t.Fatal("a wiki takes no issues")
	}
}

func TestTrackerTest(t *testing.T) {
	ts, rec := serve(t, 200, `{}`)
	for kind, path := range map[string]string{
		config.IntegrationGitHub: "/repos/a/b", config.IntegrationGitLab: "/api/v4/projects/a%2Fb",
		config.IntegrationJira: "/rest/api/2/project/OPS",
	} {
		project := "a/b"
		if kind == config.IntegrationJira {
			project = "OPS"
		}
		c, _ := New(config.Integration{Kind: kind, BaseURL: ts.URL, Project: project}, "t")
		if err := c.Test(context.Background()); err != nil || rec.method != "GET" || rec.path != path {
			t.Fatalf("%s: %v %+v", kind, err, rec)
		}
	}
}
