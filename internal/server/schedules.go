package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) listSchedules(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListSchedules(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": list})
}

func (s *Server) addSchedule(w http.ResponseWriter, r *http.Request) {
	var in service.ScheduleInput
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.AddSchedule(r.PathValue("id"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) setSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	if body.Enabled == nil {
		s.fail(w, r, &service.InvalidError{Msg: "enabled is required"})
		return
	}
	info, err := s.svc.SetScheduleEnabled(r.PathValue("id"), r.PathValue("sid"), *body.Enabled)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteSchedule(r.PathValue("id"), r.PathValue("sid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
