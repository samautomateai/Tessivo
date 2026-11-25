package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/gitdiff"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// notebookDocs extracts the notebook's enabled sources: files and folders,
// and MCP resources and web pages read now (one connection per server). A source that
// cannot be read fails the call, naming the server and URI.
func (s *Service) notebookDocs(ctx context.Context, nb *notebook.Notebook) ([]extract.Document, error) {
	if nb == nil {
		return nil, nil
	}
	clients := map[string]*mcpSession{}
	defer func() {
		for _, c := range clients {
			_ = c.Close()
			if mcpHook != nil {
				mcpHook("close")
			}
		}
	}()
	var docs []extract.Document
	for _, c := range nb.Manifest.Context {
		if !c.Enabled || c.NeedsRelink {
			continue
		}
		d, err := s.itemDocs(ctx, clients, nb, c)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d...)
	}
	return docs, nil
}

// itemDocs reads one source: a web page or an MCP resource now, a file or folder from disk.
func (s *Service) itemDocs(ctx context.Context, clients map[string]*mcpSession, nb *notebook.Notebook,
	c notebook.ContextItem,
) ([]extract.Document, error) {
	switch {
	case c.IsURL():
		d, err := fetchPage(ctx, c)
		return []extract.Document{d}, err
	case c.IsMCP():
		d, err := s.readResource(ctx, clients, c)
		return []extract.Document{d}, err
	case c.IsGitDiff():
		text, err := gitdiff.Diff(ctx, c.Path, c.URI)
		if err != nil {
			return nil, fmt.Errorf("git changes %s: %w", c.Label, err)
		}
		return []extract.Document{{Source: "git:" + c.Path + "@" + c.URI, Kind: "diff", Text: text}}, nil
	default:
		return extract.Path(nb.ContextPath(c))
	}
}

func (s *Service) readResource(ctx context.Context, clients map[string]*mcpSession, c notebook.ContextItem,
) (extract.Document, error) {
	fail := func(err error) (extract.Document, error) {
		return extract.Document{}, fmt.Errorf("mcp resource %s %s: %w", c.Server, c.URI, err)
	}
	client, ok := clients[c.Server]
	if !ok {
		var err error
		if client, err = s.connectMCP(ctx, c.Server); err != nil {
			return fail(err)
		}
		clients[c.Server] = client
		if mcpHook != nil {
			mcpHook("open")
		}
	}
	text, mime, err := client.ReadResource(ctx, c.URI)
	if err != nil {
		return fail(client.err(err))
	}
	kind := docText
	if strings.Contains(mime, "markdown") {
		kind = "markdown"
	}
	return extract.Document{Source: "mcp:" + c.Server + "/" + c.URI, Kind: kind, Text: client.redact(text)}, nil
}

// AddMCPContext attaches a configured server's resource to a notebook.
func (s *Service) AddMCPContext(notebookID, server, uri, label string) (NotebookInfo, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if _, ok := s.mcpConfig().Find(server); !ok {
		return NotebookInfo{}, s.mcpMissing(server)
	}
	if _, err := nb.AddMCPResource(server, uri, label); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	return describe(notebookID, nb, true), nil
}
