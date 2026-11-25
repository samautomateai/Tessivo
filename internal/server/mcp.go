package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/config"
)

func (s *Server) mcpRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/mcp/servers", s.listMCP)
	mux.HandleFunc("POST /v1/mcp/servers", s.addMCP)
	mux.HandleFunc("DELETE /v1/mcp/servers/{name}", s.removeMCP)
	mux.HandleFunc("POST /v1/mcp/servers/{name}/test", s.testMCP)
	mux.HandleFunc("POST /v1/mcp/servers/{name}/trust", s.trustMCP)
	mux.HandleFunc("DELETE /v1/mcp/servers/{name}/trust", s.trustMCP)
	mux.HandleFunc("PUT /v1/notebooks/{id}/mcp", s.setNotebookMCP)
	mux.HandleFunc("PUT /v1/notebooks/{id}/agent/commands", s.setNotebookCommands)
	s.integrationRoutes(mux) // outward connections, like MCP servers
}

func (s *Server) listMCP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"servers": s.svc.MCPServers()})
}

func (s *Server) addMCP(w http.ResponseWriter, r *http.Request) {
	var srv config.MCPServer
	if err := decode(w, r, &srv); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.AddMCPServer(srv)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) removeMCP(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemoveMCPServer(r.PathValue("name")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// testMCP connects to a server and lists what it offers; it takes a run slot
// because it starts a process or opens a connection.
func (s *Server) testMCP(w http.ResponseWriter, r *http.Request) {
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	res, err := s.svc.TestMCPServer(r.Context(), r.PathValue("name"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// setNotebookMCP sets the servers a notebook's agent runs may use.
func (s *Server) setNotebookMCP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Servers []string `json:"servers"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.SetNotebookMCP(r.PathValue("id"), body.Servers)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// setNotebookCommands replaces the notebook's run_command allow-list.
func (s *Server) setNotebookCommands(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Commands []config.CommandRule `json:"commands"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.SetNotebookCommands(r.PathValue("id"), body.Commands)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// trustMCP trusts (POST) or stops trusting (DELETE) a pack server; its name
// "<pack>/<server>" is sent URL-encoded.
func (s *Server) trustMCP(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.TrustMCP(r.PathValue("name"), r.Method == http.MethodPost); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
