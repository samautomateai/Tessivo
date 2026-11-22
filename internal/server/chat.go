package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/samautomateai/tessivo/internal/service"
)

func (s *Server) listChats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := s.svc.ListChats(q.Get("q"), q.Get("archived") == "true")
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (s *Server) createChat(w http.ResponseWriter, r *http.Request) {
	var in service.ChatInput
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	sess, err := s.svc.CreateChat(in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) getChat(w http.ResponseWriter, r *http.Request) {
	sess, err := s.svc.GetChat(r.PathValue("cid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) updateChat(w http.ResponseWriter, r *http.Request) {
	var p service.ChatPatch
	if err := decode(w, r, &p); err != nil {
		s.fail(w, r, err)
		return
	}
	sess, err := s.svc.UpdateChat(r.PathValue("cid"), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) deleteChat(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteChat(r.PathValue("cid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) chatMemory(w http.ResponseWriter, r *http.Request) {
	m, err := s.svc.ChatMemory()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memory": m})
}

func (s *Server) setChatMemory(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Memory []string `json:"memory"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	m, err := s.svc.SetChatMemory(body.Memory)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"memory": m})
}

// sendChat streams a reply as SSE: "delta" events, then "finished" with the
// stored turns, or "error". Closing the connection stops the reply (the
// partial text is kept, marked stopped).
func (s *Server) sendChat(w http.ResponseWriter, r *http.Request) {
	var in service.ChatSend
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.svc.CheckChatSend(r.PathValue("cid"), in); err != nil {
		s.fail(w, r, err)
		return
	}
	select {
	case s.runs <- struct{}{}:
		defer func() { <-s.runs }()
	default:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d runs or replies at once", MaxConcurrentRuns))
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	sse, ok := newSSE(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	reply, err := s.svc.SendChat(r.Context(), r.PathValue("cid"), in, func(d string) {
		sse.send("delta", map[string]string{keyText: d})
	})
	if err != nil {
		status, msg := classify(err)
		sse.send(keyError, map[string]any{keyStatus: status, keyError: msg, keyReply: reply})
		return
	}
	sse.send("finished", reply)
}

func (s *Server) planRepeatable(w http.ResponseWriter, r *http.Request) {
	plan, err := s.svc.PlanRepeatable(r.PathValue("cid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) makeRepeatable(w http.ResponseWriter, r *http.Request) {
	var in service.RepeatableInput
	if err := decode(w, r, &in); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.MakeRepeatable(r.PathValue("cid"), in)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

// saveReply restructures a reply into a validated output. It calls the model,
// so it shares the run limit and outlives the normal write timeout.
func (s *Server) saveReply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WorkType string `json:"workType"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	select {
	case s.runs <- struct{}{}:
		defer func() { <-s.runs }()
	default:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d runs or replies at once", MaxConcurrentRuns))
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
	out, err := s.svc.SaveChatReply(r.Context(), r.PathValue("cid"), r.PathValue("mid"), body.WorkType)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
