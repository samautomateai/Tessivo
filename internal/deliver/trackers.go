package deliver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
)

// CreateIssue files one issue in the tracker.
func (c *Client) CreateIssue(ctx context.Context, is Issue) (Ref, error) {
	switch c.in.Kind {
	case config.IntegrationGitHub:
		return c.githubIssue(ctx, is)
	case config.IntegrationGitLab:
		return c.gitlabIssue(ctx, is)
	case config.IntegrationJira:
		return c.jiraIssue(ctx, is)
	}
	return Ref{}, fmt.Errorf("%s: %w", c.in.Kind, errNotTracker)
}

func (c *Client) githubIssue(ctx context.Context, is Issue) (Ref, error) {
	var out struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	body := map[string]any{keyTitle: is.Title, "body": is.Body, fieldLabels: nonNil(is.Labels)}
	if err := c.do(ctx, http.MethodPost, "/repos/"+c.in.Project+"/issues", body, &out); err != nil {
		return Ref{}, err
	}
	return Ref{ID: strconv.Itoa(out.Number), URL: out.HTMLURL}, nil
}

func (c *Client) gitlabIssue(ctx context.Context, is Issue) (Ref, error) {
	var out struct {
		IID    int    `json:"iid"`
		WebURL string `json:"web_url"`
	}
	labels := make([]string, 0, len(is.Labels))
	for _, l := range is.Labels {
		labels = append(labels, strings.ReplaceAll(l, ",", " "))
	}
	body := map[string]any{keyTitle: is.Title, "description": is.Body, fieldLabels: strings.Join(labels, ",")}
	if err := c.do(ctx, http.MethodPost, c.gitlabProject()+"/issues", body, &out); err != nil {
		return Ref{}, err
	}
	return Ref{ID: strconv.Itoa(out.IID), URL: out.WebURL}, nil
}

func (c *Client) gitlabProject() string { return "/api/v4/projects/" + url.PathEscape(c.in.Project) }

func (c *Client) jiraIssue(ctx context.Context, is Issue) (Ref, error) {
	var out struct {
		Key string `json:"key"`
	}
	kind := c.in.IssueType
	if kind == "" {
		kind = "Task"
	}
	labels := make([]string, 0, len(is.Labels))
	for _, l := range is.Labels {
		labels = append(labels, strings.Join(strings.Fields(l), "-")) // Jira labels take no spaces
	}
	body := map[string]any{"fields": map[string]any{
		"project": map[string]string{"key": c.in.Project}, "summary": is.Title, "description": is.Body,
		"issuetype": map[string]string{"name": kind}, fieldLabels: labels,
	}}
	if err := c.do(ctx, http.MethodPost, "/rest/api/2/issue", body, &out); err != nil {
		return Ref{}, err
	}
	return Ref{ID: out.Key, URL: c.in.URL() + "/browse/" + url.PathEscape(out.Key)}, nil
}

// Test checks that the target exists and the token may read it.
func (c *Client) Test(ctx context.Context) error {
	var path string
	switch c.in.Kind {
	case config.IntegrationGitHub:
		path = "/repos/" + c.in.Project
	case config.IntegrationGitLab:
		path = c.gitlabProject()
	case config.IntegrationJira:
		path = "/rest/api/2/project/" + url.PathEscape(c.in.Project)
	case config.IntegrationConfluence:
		path = "/rest/api/space/" + url.PathEscape(c.in.Space)
	case config.IntegrationNotion:
		path = "/v1/pages/" + url.PathEscape(c.in.Parent)
	case config.IntegrationFolder:
		return c.testFolder()
	default:
		return fmt.Errorf("unknown integration kind %q", c.in.Kind)
	}
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

const fieldLabels = "labels"

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
