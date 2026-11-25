// Package deliver sends outputs where people work (REP-4, REP-5): issues to
// GitHub, GitLab or Jira, pages to Confluence or Notion, and files to a
// folder. Every request is bounded (30 s, 1 MiB answers, no redirects) and an
// error never carries the token.
package deliver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// Limits on each request.
const (
	Timeout     = 30 * time.Second
	MaxResponse = 1 << 20
	maxErrText  = 300
)

// Issue is what a tracker gets: a title, a Markdown body and labels.
type Issue struct {
	Title  string
	Body   string
	Labels []string
}

// Ref is what was created: its ID in the target and a link to open it.
type Ref struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Client talks to one integration with its token.
type Client struct {
	in    config.Integration
	token string
	http  *http.Client
}

// New returns a client for an integration; token may be empty for a folder.
func New(in config.Integration, token string) (*Client, error) {
	return &Client{in: in, token: token, http: &http.Client{
		Timeout:       Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// do sends one JSON request to base+path and decodes a JSON answer into out (when not nil).
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s: %w", c.in.Kind, err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.in.URL()+path, r)
	if err != nil {
		return fmt.Errorf("%s: %w", c.in.Kind, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Tessivo")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %s", c.in.Kind, c.clean(err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponse+1))
	if err != nil {
		return fmt.Errorf("%s: read the answer: %s", c.in.Kind, c.clean(err.Error()))
	}
	if len(data) > MaxResponse {
		return fmt.Errorf("%s: the answer is too large (over %d bytes)", c.in.Kind, MaxResponse)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: %s: %s", c.in.Kind, resp.Status, c.clean(string(data)))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: unexpected answer: %w", c.in.Kind, err)
	}
	return nil
}

// auth sets the kind's credentials header.
func (c *Client) auth(req *http.Request) {
	switch {
	case c.token == "":
	case c.in.Kind == config.IntegrationGitLab:
		req.Header.Set("PRIVATE-TOKEN", c.token)
	case c.in.User != "" && (c.in.Kind == config.IntegrationJira || c.in.Kind == config.IntegrationConfluence):
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.in.User+":"+c.token)))
	default:
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	switch c.in.Kind {
	case config.IntegrationGitHub:
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	case config.IntegrationNotion:
		req.Header.Set("Notion-Version", "2022-06-28")
	}
}

// clean removes the token and anything secret-looking, and cuts the text short.
func (c *Client) clean(s string) string {
	s = secrets.RedactPatterns(secrets.Redact(s, c.token))
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxErrText {
		s = string(r[:maxErrText]) + "…"
	}
	return s
}

var errNotTracker = errors.New("this integration takes pages, not issues")
