package server

import (
	"bytes"
	"log/slog"
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/settings", s.updateSettings)
	mux.HandleFunc("GET /v1/secrets/{ref}", s.secretStatus)
	mux.HandleFunc("PUT /v1/secrets/{ref}", s.setSecret)
	mux.HandleFunc("DELETE /v1/secrets/{ref}", s.deleteSecret)
	mux.HandleFunc("GET /v1/diagnostics", s.diagnostics)
	mux.HandleFunc("POST /v1/models/test", s.testModel)
	s.profileRoutes(mux)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var body service.SettingsUpdate
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	f, err := s.svc.UpdateSettings(body)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// secretStatus says whether a key is set; its value never leaves the core.
func (s *Server) secretStatus(w http.ResponseWriter, r *http.Request) {
	ok, err := s.svc.SecretSet(r.PathValue("ref"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"set": ok})
}

func (s *Server) setSecret(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value string `json:"value"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.svc.SetSecret(r.PathValue("ref"), body.Value); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteSecret(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteSecret(r.PathValue("ref")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetLogger sends the server's log (start, stop, failures, events) to l.
func (s *Server) SetLogger(l *slog.Logger) {
	s.log = l
	s.log.Info("core started", "version", s.version)
}

// Logger is the server's log.
func (s *Server) Logger() *slog.Logger { return s.log }

func (s *Server) diagnostics(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	if err := s.svc.Diagnostics(&buf, s.version, s.svc.LogPath); err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="tessivo-diagnostics.zip"`)
	_, _ = w.Write(buf.Bytes())
}

// testModel sends one tiny request to every model behind an alias or provider/model.
func (s *Server) testModel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	checks, err := s.svc.TestModel(r.Context(), body.Model)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": checks})
}
