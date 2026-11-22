package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("PUT /v1/settings", s.updateSettings)
	mux.HandleFunc("GET /v1/secrets/{ref}", s.secretStatus)
	mux.HandleFunc("PUT /v1/secrets/{ref}", s.setSecret)
	mux.HandleFunc("DELETE /v1/secrets/{ref}", s.deleteSecret)
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
