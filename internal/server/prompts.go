package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/notebook"
)

func (s *Server) promptRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notebooks/{id}/prompts", s.listPrompts)
	mux.HandleFunc("PUT /v1/notebooks/{id}/prompts/{name}", s.savePrompt)
	mux.HandleFunc("DELETE /v1/notebooks/{id}/prompts/{name}", s.deletePrompt)
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
