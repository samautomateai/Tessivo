package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) notebookManageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PATCH /v1/notebooks/{id}", s.updateNotebook)
	mux.HandleFunc("DELETE /v1/notebooks/{id}", s.deleteNotebook)
}

func (s *Server) updateNotebook(w http.ResponseWriter, r *http.Request) {
	var body service.NotebookUpdate
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.UpdateNotebook(r.PathValue("id"), body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) deleteNotebook(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteNotebook(r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
