package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/config"
)

func (s *Server) integrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/integrations", s.listIntegrations)
	mux.HandleFunc("POST /v1/integrations", s.addIntegration)
	mux.HandleFunc("DELETE /v1/integrations/{name}", s.removeIntegration)
}

func (s *Server) listIntegrations(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"integrations": s.svc.ListIntegrations()})
}

func (s *Server) addIntegration(w http.ResponseWriter, r *http.Request) {
	var in config.Integration
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.AddIntegration(in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) removeIntegration(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemoveIntegration(r.PathValue("name")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
