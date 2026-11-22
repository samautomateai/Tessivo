package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/samautomateai/tessivo/internal/agenttools"
	"github.com/samautomateai/tessivo/internal/mcp"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// maxAttachedMCP bounds the servers one notebook attaches.
const maxAttachedMCP = 20

// mcpHook, when set by a test, sees each MCP connection open and close.
var mcpHook func(event string)

// mcpCaller adapts an MCP client to the agent's tool interface; errors are
// redacted before the model or the run record sees them.
type mcpCaller struct {
	c      *mcp.Client
	redact func(string) string
}

func (m mcpCaller) CallTool(ctx context.Context, name string, args json.RawMessage) (string, bool, error) {
	res, err := m.c.CallTool(ctx, name, args)
	if err != nil {
		return "", false, errors.New(m.redact(err.Error()))
	}
	return m.redact(res.Text), res.IsError, nil // a server may echo its own credentials
}

// mcpTools connects to the notebook's attached servers and returns their
// tools. closeAll stops every connection; call it when the run ends or
// pauses. A server that cannot be reached fails the call, naming it.
func (s *Service) mcpTools(ctx context.Context, nb *notebook.Notebook,
) (tools []agenttools.Tool, closeAll func(), err error) {
	var clients []*mcpSession
	closeAll = func() {
		for _, c := range clients {
			_ = c.Close()
			if mcpHook != nil {
				mcpHook("close")
			}
		}
	}
	taken := map[string]bool{} // tool names stay unique across servers
	for _, name := range nb.Manifest.Agent.MCPServers {
		c, err := s.connectMCP(ctx, name)
		if err != nil {
			closeAll()
			return nil, func() {}, fmt.Errorf("attached MCP server %s: %w", name, err)
		}
		clients = append(clients, c)
		if mcpHook != nil {
			mcpHook("open")
		}
		list, err := c.ListTools(ctx)
		if err != nil {
			closeAll()
			return nil, func() {}, fmt.Errorf("attached MCP server %s: %w", name, c.err(err))
		}
		specs := make([]agenttools.MCPToolSpec, 0, len(list))
		for _, t := range list {
			specs = append(specs, agenttools.MCPToolSpec{
				Server: name, Name: t.Name, Description: t.Description, Schema: t.InputSchema,
			})
		}
		tools = append(tools, agenttools.MCPTools(name, specs, mcpCaller{c: c.Client, redact: c.redact}, taken)...)
	}
	return tools, closeAll, nil
}

// SetNotebookMCP sets the servers a notebook's agent runs may use. Every
// name must be a configured server; duplicates are dropped.
func (s *Service) SetNotebookMCP(notebookID string, servers []string) (NotebookInfo, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return NotebookInfo{}, err
	}
	cfg := s.mcpConfig()
	var list []string
	for _, name := range servers {
		if _, ok := cfg.Find(name); !ok {
			return NotebookInfo{}, invalidf("mcp server %q is not configured; add it first", name)
		}
		if !slices.Contains(list, name) {
			list = append(list, name)
		}
	}
	if len(list) > maxAttachedMCP {
		return NotebookInfo{}, invalidf("at most %d MCP servers per notebook", maxAttachedMCP)
	}
	nb.Manifest.Agent.MCPServers = list
	if err := nb.Save(); err != nil {
		return NotebookInfo{}, err
	}
	return describe(notebookID, nb, false), nil
}
