package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/mcp"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// notebookDocs extracts the notebook's enabled sources: files and folders,
// and MCP resources read now (one connection per server). A resource that
// cannot be read fails the call, naming the server and URI.
func (s *Service) notebookDocs(ctx context.Context, nb *notebook.Notebook) ([]extract.Document, error) {
	if nb == nil {
		return nil, nil
	}
	clients := map[string]*mcp.Client{}
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
		if !c.Enabled {
			continue
		}
		if !c.IsMCP() {
			d, err := extract.Path(nb.ContextPath(c))
			if err != nil {
				return nil, err
			}
			docs = append(docs, d...)
			continue
		}
		d, err := s.readResource(ctx, clients, c)
		if err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (s *Service) readResource(ctx context.Context, clients map[string]*mcp.Client, c notebook.ContextItem,
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
		return fail(s.mcpErr(err))
	}
	kind := docText
	if strings.Contains(mime, "markdown") {
		kind = "markdown"
	}
	return extract.Document{Source: "mcp:" + c.Server + "/" + c.URI, Kind: kind, Text: text}, nil
}

// AddMCPContext attaches a configured server's resource to a notebook.
func (s *Service) AddMCPContext(notebookID, server, uri, label string) (NotebookInfo, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	if _, ok := s.mcpConfig().Find(server); !ok {
		return NotebookInfo{}, invalidf("mcp server %q is not configured; add it first", server)
	}
	if _, err := nb.AddMCPResource(server, uri, label); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	return describe(notebookID, nb, true), nil
}
