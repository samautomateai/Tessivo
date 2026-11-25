package server

import (
	"net/http"
)

func (s *Server) deliverRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/notebooks/{id}/outputs/{out}/issues/push", s.pushIssues)
	mux.HandleFunc("POST /v1/notebooks/{id}/outputs/{out}/publish", s.publishOutput)
	mux.HandleFunc("GET /v1/notebooks/{id}/outputs/{out}/deliveries", s.listDeliveries)
	mux.HandleFunc("POST /v1/integrations/{name}/test", s.testIntegration)
}

// pushIssues files findings as issues. A failure after some were sent still
// reports what was sent, with the error.
func (s *Server) pushIssues(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Integration string `json:"integration"`
		Findings    []int  `json:"findings"` // 0-based; empty: all
		Again       bool   `json:"again"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	res, err := s.svc.PushIssues(r.Context(), r.PathValue("id"), r.PathValue("out"), body.Integration, body.Findings,
		body.Again)
	if err != nil && len(res.Sent) > 0 {
		status, msg := classify(err)
		writeJSON(w, status, map[string]any{"error": msg, "sent": res.Sent, "skipped": res.Skipped})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) publishOutput(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Integration string `json:"integration"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	d, err := s.svc.Publish(r.Context(), r.PathValue("id"), r.PathValue("out"), body.Integration)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) listDeliveries(w http.ResponseWriter, r *http.Request) {
	ds, err := s.svc.Deliveries(r.PathValue("id"), r.PathValue("out"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deliveries": ds})
}

func (s *Server) testIntegration(w http.ResponseWriter, r *http.Request) {
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	if err := s.svc.TestIntegration(r.Context(), r.PathValue("name")); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
