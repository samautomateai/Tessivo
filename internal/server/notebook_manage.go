package server

import (
	"encoding/json"
	"mime"
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) notebookManageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PATCH /v1/notebooks/{id}", s.updateNotebook)
	mux.HandleFunc("DELETE /v1/notebooks/{id}", s.deleteNotebook)
	mux.HandleFunc("GET /v1/notebooks/{id}/outputs/{out}/export", s.exportOutput)
	mux.HandleFunc("GET /v1/notebooks/{id}/outputs/{out}/run", s.outputRun)
	mux.HandleFunc("GET /v1/notebooks/{id}/outputs/{out}/diff", s.diffOutput)
	mux.HandleFunc("POST /v1/notebooks/{id}/outputs/{out}/edits", s.editOutput)
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

func (s *Server) exportOutput(w http.ResponseWriter, r *http.Request) {
	f, err := s.svc.ExportOutput(r.PathValue("id"), r.PathValue("out"), r.URL.Query().Get("format"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(f.Data)
}

func (s *Server) outputRun(w http.ResponseWriter, r *http.Request) {
	ri, err := s.svc.OutputRun(r.PathValue("id"), r.PathValue("out"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ri)
}

func (s *Server) diffOutput(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.DiffOutputs(r.PathValue("id"), r.PathValue("out"), r.URL.Query().Get("against"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// editOutput saves the edited structured result as a new version.
func (s *Server) editOutput(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data json.RawMessage `json:"data"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.EditOutput(r.PathValue("id"), r.PathValue("out"), body.Data)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}
