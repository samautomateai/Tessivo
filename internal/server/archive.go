package server

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/samautomateai/tessivo/internal/archive"
	"github.com/samautomateai/tessivo/internal/service"
)

// maxImportBytes caps an uploaded archive (its compressed size).
const maxImportBytes = archive.MaxTotal

func (s *Server) archiveRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/notebooks/{id}/archive", s.exportNotebook)
	mux.HandleFunc("POST /v1/notebooks/import", s.importNotebook)
	mux.HandleFunc("GET /v1/research/{id}/archive", s.exportResearch)
	mux.HandleFunc("POST /v1/research/import", s.importResearch)
	mux.HandleFunc("POST /v1/notebooks/{id}/context/{ctx}/relink", s.relinkContext)
}

func (s *Server) exportNotebook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	outputs := r.URL.Query().Get("outputs") == "1"
	s.sendArchive(w, r, id, func(out io.Writer) error { return s.svc.ExportNotebook(id, outputs, out) })
}

func (s *Server) exportResearch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.sendArchive(w, r, id, func(out io.Writer) error { return s.svc.ExportResearch(id, out) })
}

// sendArchive builds the archive in a temporary file first, so a failure is
// still a proper error response, then streams it.
func (s *Server) sendArchive(w http.ResponseWriter, r *http.Request, id string, write func(io.Writer) error) {
	f, err := os.CreateTemp("", "tessivo-archive-*.zip")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	if err := write(f); err != nil {
		s.fail(w, r, err)
		return
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": id + ".tessivo.zip"}))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (s *Server) importNotebook(w http.ResponseWriter, r *http.Request) {
	s.receiveArchive(w, r, func(f *os.File, size int64) (any, error) { return s.svc.ImportNotebook(f, size) })
}

func (s *Server) importResearch(w http.ResponseWriter, r *http.Request) {
	s.receiveArchive(w, r, func(f *os.File, size int64) (any, error) { return s.svc.ImportResearch(f, size) })
}

// receiveArchive spools an application/zip body (at most maxImportBytes) to a
// private temporary file and imports it.
func (s *Server) receiveArchive(w http.ResponseWriter, r *http.Request, imp func(*os.File, int64) (any, error)) {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/zip" {
		s.fail(w, r, &service.InvalidError{Msg: "Content-Type must be application/zip"})
		return
	}
	f, err := os.CreateTemp("", "tessivo-import-*.zip")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, maxImportBytes))
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		s.fail(w, r, &service.InvalidError{Msg: fmt.Sprintf("archive is larger than %d MB", maxImportBytes>>20)})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := imp(f, n)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) relinkContext(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.RelinkContext(r.PathValue("id"), r.PathValue("ctx"), body.Path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}
