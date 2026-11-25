package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) profileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/profiles", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.svc.Profiles())
	})
	mux.HandleFunc("POST /v1/profiles", s.saveProfile)
	mux.HandleFunc("POST /v1/profiles/{name}/use", func(w http.ResponseWriter, r *http.Request) {
		s.profileResult(w, r)(s.svc.UseProfile(r.PathValue("name")))
	})
	mux.HandleFunc("DELETE /v1/profiles/{name}", func(w http.ResponseWriter, r *http.Request) {
		s.profileResult(w, r)(s.svc.DeleteProfile(r.PathValue("name")))
	})
}

func (s *Server) saveProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	s.profileResult(w, r)(s.svc.SaveProfile(body.Name))
}

// profileResult writes the profiles after a change, or the error.
func (s *Server) profileResult(w http.ResponseWriter, r *http.Request) func(service.ProfilesInfo, error) {
	return func(info service.ProfilesInfo, err error) {
		if err != nil {
			s.fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, info)
	}
}
