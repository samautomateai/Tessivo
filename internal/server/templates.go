package server

import (
	"net/http"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) listTemplates(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListTemplates(r.URL.Query().Get("workType"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": list})
}

func (s *Server) getTemplate(w http.ResponseWriter, r *http.Request) {
	t, err := s.svc.GetTemplate(r.PathValue("tid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// createTemplate saves a notebook as a template or extends an existing one.
func (s *Server) createTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FromNotebook string `json:"fromNotebook"`
		Extend       string `json:"extend"`
		ID           string `json:"id"`
		Name         string `json:"name"`
		Description  string `json:"description"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	var (
		t   service.TemplateInfo
		err error
	)
	switch {
	case body.FromNotebook != "" && body.Extend == "":
		t, err = s.svc.SaveAsTemplate(body.FromNotebook, body.ID, body.Name, body.Description)
	case body.Extend != "" && body.FromNotebook == "":
		t, err = s.svc.ExtendTemplate(body.Extend, body.ID, body.Name)
	default:
		err = &service.InvalidError{Msg: "give exactly one of fromNotebook or extend"}
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteTemplate(r.PathValue("tid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setMemory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Memory []string `json:"memory"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.SetMemory(r.PathValue("id"), body.Memory)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}
