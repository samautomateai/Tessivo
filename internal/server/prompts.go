package server

import (
	"net/http"
	"strconv"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) promptRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notebooks/{id}/prompts", s.listPrompts)
	mux.HandleFunc("PUT /v1/notebooks/{id}/prompts/{name}", s.savePrompt)
	mux.HandleFunc("DELETE /v1/notebooks/{id}/prompts/{name}", s.deletePrompt)
	mux.HandleFunc("GET /v1/notebooks/{id}/prompts/{name}/versions", s.promptVersions)
	mux.HandleFunc("POST /v1/notebooks/{id}/prompts/{name}/versions/{v}/restore", s.restorePrompt)
	mux.HandleFunc("GET /v1/usage", s.usage)
	mux.HandleFunc("GET /v1/search", s.search)
}

func (s *Server) listPrompts(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListPrompts(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prompts": list})
}

func (s *Server) savePrompt(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text     string             `json:"text"`
		Settings config.RunSettings `json:"settings"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.svc.SavePrompt(r.PathValue("id"), notebook.Prompt{
		Name: r.PathValue("name"), Text: body.Text, Settings: body.Settings,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deletePrompt(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeletePrompt(r.PathValue("id"), r.PathValue("name")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.Usage()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	res, err := s.svc.Search(r.URL.Query().Get("q"), limit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": res})
}

func (s *Server) promptVersions(w http.ResponseWriter, r *http.Request) {
	vs, err := s.svc.PromptVersions(r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": vs})
}

func (s *Server) restorePrompt(w http.ResponseWriter, r *http.Request) {
	v, err := strconv.Atoi(r.PathValue("v"))
	if err != nil || v < 1 {
		s.fail(w, r, &service.InvalidError{Msg: "version must be a number from 1"})
		return
	}
	p, err := s.svc.RestorePromptVersion(r.PathValue("id"), r.PathValue("name"), v)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
