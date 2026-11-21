package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

// Output thread routes, relative to /v1/notebooks/{id}/outputs/{out}.
func (s *Server) threadRoutes(mux *http.ServeMux) {
	const p = "/v1/notebooks/{id}/outputs/{out}"
	mux.HandleFunc("GET "+p+"/thread", s.getThread)
	mux.HandleFunc("PATCH "+p+"/thread", s.patchThread)
	mux.HandleFunc("POST "+p+"/thread/messages", s.sendThread)
	mux.HandleFunc("GET "+p+"/suggestions", s.listSuggestions)
	mux.HandleFunc("POST "+p+"/suggestions/generate", s.generateSuggestions)
	mux.HandleFunc("POST "+p+"/suggestions/{sid}/apply", s.applySuggestion)
	mux.HandleFunc("POST "+p+"/suggestions/{sid}/dismiss", s.dismissSuggestion)
	mux.HandleFunc("POST "+p+"/versions", s.newVersion)
}

func ids(r *http.Request) (notebookID, outputID string) { return r.PathValue("id"), r.PathValue("out") }

func (s *Server) getThread(w http.ResponseWriter, r *http.Request) {
	nb, out := ids(r)
	sess, err := s.svc.GetThread(nb, out)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) patchThread(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model          string `json:"model"`
		ThinkingEffort string `json:"thinkingEffort"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	nb, out := ids(r)
	sess, err := s.svc.SetThreadSettings(nb, out, body.Model, body.ThinkingEffort)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// takeSlot reserves one of the run slots, or answers 429.
func (s *Server) takeSlot(w http.ResponseWriter) (release func(), ok bool) {
	select {
	case s.runs <- struct{}{}:
		return func() { <-s.runs }, true
	default:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d runs or replies at once", MaxConcurrentRuns))
		return nil, false
	}
}

// sendThread streams a thread reply: delta, suggestion and finished events,
// or error with the reply so far.
func (s *Server) sendThread(w http.ResponseWriter, r *http.Request) {
	var in service.ChatSend
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	nb, out := ids(r)
	if err := s.svc.CheckThreadSend(nb, out, in); err != nil {
		s.fail(w, r, err)
		return
	}
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	sse, ok := newSSE(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	reply, err := s.svc.SendThread(r.Context(), nb, out, in,
		func(d string) { sse.send("delta", map[string]string{"text": d}) },
		func(sg notebook.Suggestion) { sse.send("suggestion", sg) })
	if err != nil {
		status, msg := classify(err)
		sse.send(keyError, map[string]any{keyStatus: status, keyError: msg, "reply": reply})
		return
	}
	sse.send("finished", reply)
}

func (s *Server) listSuggestions(w http.ResponseWriter, r *http.Request) {
	nb, out := ids(r)
	list, err := s.svc.ListSuggestions(nb, out)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": list})
}

func (s *Server) generateSuggestions(w http.ResponseWriter, r *http.Request) {
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	nb, out := ids(r)
	list, err := s.svc.GenerateSuggestions(r.Context(), nb, out)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestions": list})
}

func (s *Server) applySuggestion(w http.ResponseWriter, r *http.Request) {
	nb, out := ids(r)
	res, err := s.svc.ApplySuggestion(nb, out, r.PathValue("sid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) dismissSuggestion(w http.ResponseWriter, r *http.Request) {
	nb, out := ids(r)
	sg, err := s.svc.DismissSuggestion(nb, out, r.PathValue("sid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"suggestion": sg})
}

// newVersion revises the output from its thread and answers with the new
// output (201), or 422 when the model's revision fails validation.
func (s *Server) newVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FromMessageID string `json:"fromMessageId"`
	}
	if r.ContentLength != 0 {
		if err := decode(w, r, &body); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	release, ok := s.takeSlot(w)
	if !ok {
		return
	}
	defer release()
	nb, out := ids(r)
	res, err := s.svc.NewVersion(r.Context(), nb, out, body.FromMessageID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.GetOutput(nb, res.OutputID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}
