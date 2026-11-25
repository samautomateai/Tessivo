package server

import (
	"context"
	"net/http"

	"github.com/samautomateai/tessivo/internal/pack"
)

func (s *Server) packRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/packs", s.listPacks)
	mux.HandleFunc("POST /v1/packs", s.addPack)
	mux.HandleFunc("POST /v1/packs/refresh", s.refreshPacks)
	mux.HandleFunc("DELETE /v1/packs/{id}", s.removePack)
}

func (s *Server) listPacks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"packs": s.svc.Packs()})
}

func (s *Server) addPack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
		Branch string `json:"branch"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), pack.GitTimeout)
	defer cancel()
	info, err := s.svc.AddPack(ctx, body.Source, body.Branch)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) refreshPacks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"packs": s.svc.RefreshPacks(r.Context())})
}

func (s *Server) removePack(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RemovePack(r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
