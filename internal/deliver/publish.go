package deliver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/atomicfile"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/export"
)

// Page is an output to publish. PageID is what an earlier publish of the same
// output created (a file name or page ID), so publishing again replaces it.
type Page struct {
	Title    string
	Markdown string
	PageID   string
}

// Bounds on a published page.
const (
	maxNotionBlocks = 1000 // more are cut, with a note
	notionChunk     = 100  // Notion takes at most 100 children per request
	maxFolderTries  = 100
)

var errNotWiki = errors.New("this integration takes issues, not pages")

// Publish writes the page to the folder, Confluence or Notion.
func (c *Client) Publish(ctx context.Context, p Page) (Ref, error) {
	switch c.in.Kind {
	case config.IntegrationFolder:
		return c.folderPage(p)
	case config.IntegrationConfluence:
		return c.confluencePage(ctx, p)
	case config.IntegrationNotion:
		return c.notionPage(ctx, p)
	}
	return Ref{}, fmt.Errorf("%s: %w", c.in.Kind, errNotWiki)
}

// folderPage writes <title>.md. A file this output wrote before is replaced;
// any other file of that name is left alone and a numbered name is used.
func (c *Client) folderPage(p Page) (Ref, error) {
	if info, err := os.Stat(c.in.Path); err != nil || !info.IsDir() {
		return Ref{}, fmt.Errorf("folder: %s is not a folder", c.in.Path)
	}
	name := ""
	if p.PageID != "" && filepath.IsLocal(p.PageID) && filepath.Base(p.PageID) == p.PageID {
		name = p.PageID
	}
	base := strings.TrimSuffix(export.FileName(p.Title, "md"), ".md")
	for i := 1; name == "" && i <= maxFolderTries; i++ {
		try := base + ".md"
		if i > 1 {
			try = fmt.Sprintf("%s (%d).md", base, i)
		}
		if _, err := os.Lstat(filepath.Join(c.in.Path, try)); errors.Is(err, os.ErrNotExist) {
			name = try
		}
	}
	if name == "" {
		return Ref{}, fmt.Errorf("folder: too many files named %s", base)
	}
	path := filepath.Join(c.in.Path, name)
	if err := atomicfile.Write(path, []byte(p.Markdown), 0o600); err != nil {
		return Ref{}, fmt.Errorf("folder: %w", err)
	}
	return Ref{ID: name, URL: (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()}, nil
}

type confluenceAnswer struct {
	ID      string `json:"id"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Links struct {
		Base  string `json:"base"`
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// confluencePage creates the page, or updates the one this output published before.
func (c *Client) confluencePage(ctx context.Context, p Page) (Ref, error) {
	body := map[string]any{
		keyType: "page", keyTitle: p.Title, "space": map[string]string{"key": c.in.Space},
		"body": map[string]any{"storage": map[string]string{"value": HTML(p.Markdown), "representation": "storage"}},
	}
	var out confluenceAnswer
	var err error
	if p.PageID == "" {
		if c.in.Parent != "" {
			body["ancestors"] = []map[string]string{{"id": c.in.Parent}}
		}
		err = c.do(ctx, http.MethodPost, "/rest/api/content", body, &out)
	} else {
		out, err = c.confluenceUpdate(ctx, p.PageID, body)
	}
	if err != nil {
		return Ref{}, err
	}
	return Ref{ID: out.ID, URL: out.Links.Base + out.Links.WebUI}, nil
}

// confluenceUpdate replaces a page's body, one version after the current one.
func (c *Client) confluenceUpdate(ctx context.Context, id string, body map[string]any) (confluenceAnswer, error) {
	page := "/rest/api/content/" + url.PathEscape(id)
	var cur, out confluenceAnswer
	if err := c.do(ctx, http.MethodGet, page+"?expand=version", nil, &cur); err != nil {
		return out, err
	}
	body["id"], body["version"] = id, map[string]int{"number": cur.Version.Number + 1}
	err := c.do(ctx, http.MethodPut, page, body, &out)
	return out, err
}

// notionPage creates a child page of the parent and appends its blocks 100 at a time.
func (c *Client) notionPage(ctx context.Context, p Page) (Ref, error) {
	blocks := NotionBlocks(p.Markdown)
	if len(blocks) > maxNotionBlocks {
		blocks = append(blocks[:maxNotionBlocks-1], NotionBlocks("(Cut: the rest is in Tessivo.)")...)
	}
	first := blocks[:min(len(blocks), notionChunk)]
	body := map[string]any{
		"parent":     map[string]string{"page_id": c.in.Parent},
		"properties": map[string]any{keyTitle: map[string]any{keyTitle: richText([]span{{text: p.Title}})}},
		"children":   first,
	}
	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := c.do(ctx, http.MethodPost, "/v1/pages", body, &out); err != nil {
		return Ref{}, err
	}
	for rest := blocks[len(first):]; len(rest) > 0; {
		n := min(len(rest), notionChunk)
		path := "/v1/blocks/" + url.PathEscape(out.ID) + "/children"
		if err := c.do(ctx, http.MethodPatch, path, map[string]any{"children": rest[:n]}, nil); err != nil {
			return Ref{ID: out.ID, URL: out.URL}, fmt.Errorf("the page was created but is incomplete: %w", err)
		}
		rest = rest[n:]
	}
	return Ref{ID: out.ID, URL: out.URL}, nil
}
