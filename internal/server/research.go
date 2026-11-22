package server

import (
	"net/http"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/research"
	"github.com/samautomateai/tessivo/internal/service"
)

// Research notebook routes (docs/api.md, "Research notebooks").
func (s *Server) researchRoutes(mux *http.ServeMux) {
	const p = "/v1/research"
	mux.HandleFunc("GET "+p, s.listResearch)
	mux.HandleFunc("POST "+p, s.createResearch)
	mux.HandleFunc("GET "+p+"/{id}", s.getResearch)
	mux.HandleFunc("PATCH "+p+"/{id}", s.updateResearch)
	mux.HandleFunc("DELETE "+p+"/{id}", s.deleteResearch)
	mux.HandleFunc("POST "+p+"/{id}/sources", s.addResearchSource)
	mux.HandleFunc("GET "+p+"/{id}/sources/{sid}", s.getResearchSource)
	mux.HandleFunc("PATCH "+p+"/{id}/sources/{sid}", s.setSourceLevel)
	mux.HandleFunc("DELETE "+p+"/{id}/sources/{sid}", s.removeResearchSource)
	mux.HandleFunc("POST "+p+"/{id}/sources/{sid}/summary", s.refreshSummary)
	mux.HandleFunc("GET "+p+"/{id}/context", s.researchContext)
	s.researchChatRoutes(mux, p)
}

func (s *Server) listResearch(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListResearch()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": list})
}

func (s *Server) createResearch(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name, Description string }
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.CreateResearch(body.Name, body.Description)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) getResearch(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.GetResearch(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) updateResearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        *string             `json:"name"`
		Description *string             `json:"description"`
		Settings    *config.RunSettings `json:"settings"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.UpdateResearch(r.PathValue("id"), body.Name, body.Description, body.Settings)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) deleteResearch(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteResearch(r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addResearchSource(w http.ResponseWriter, r *http.Request) {
	var body struct{ Path, Server, URI string }
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.AddResearchSource(r.Context(), r.PathValue("id"), body.Path, body.Server, body.URI, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) getResearchSource(w http.ResponseWriter, r *http.Request) {
	text, summary, err := s.svc.ResearchSource(r.Context(), r.PathValue("id"), r.PathValue("sid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{keyText: text, "summary": summary})
}

func (s *Server) setSourceLevel(w http.ResponseWriter, r *http.Request) {
	var body struct{ Level string }
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.SetSourceLevel(r.PathValue("id"), r.PathValue("sid"), body.Level)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) removeResearchSource(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.RemoveResearchSource(r.PathValue("id"), r.PathValue("sid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) refreshSummary(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.RefreshSummary(r.Context(), r.PathValue("id"), r.PathValue("sid"), false); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{keyStatus: "queued"})
}

func (s *Server) researchContext(w http.ResponseWriter, r *http.Request) {
	tot, err := s.svc.ResearchContext(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tot)
}

func (s *Server) researchChatRoutes(mux *http.ServeMux, p string) {
	mux.HandleFunc("GET "+p+"/{id}/chats", s.listResearchChats)
	mux.HandleFunc("POST "+p+"/{id}/chats", s.createResearchChat)
	mux.HandleFunc("GET "+p+"/{id}/chats/{cid}", s.getResearchChat)
	mux.HandleFunc("PATCH "+p+"/{id}/chats/{cid}", s.updateResearchChat)
	mux.HandleFunc("DELETE "+p+"/{id}/chats/{cid}", s.deleteResearchChat)
	mux.HandleFunc("POST "+p+"/{id}/chats/{cid}/messages", s.sendResearch)
	mux.HandleFunc("POST "+p+"/{id}/chats/{cid}/messages/{mid}/note", s.noteFromMessage)
	mux.HandleFunc("GET "+p+"/{id}/notes", s.listResearchNotes)
	mux.HandleFunc("POST "+p+"/{id}/notes", s.saveResearchNote)
	mux.HandleFunc("PATCH "+p+"/{id}/notes/{nid}", s.saveResearchNote)
	mux.HandleFunc("DELETE "+p+"/{id}/notes/{nid}", s.deleteResearchNote)
}

func (s *Server) listResearchChats(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListResearchChats(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (s *Server) createResearchChat(w http.ResponseWriter, r *http.Request) {
	var body struct{ Title string }
	if r.ContentLength != 0 {
		if err := decode(w, r, &body); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	sess, err := s.svc.CreateResearchChat(r.PathValue("id"), body.Title)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) getResearchChat(w http.ResponseWriter, r *http.Request) {
	sess, cites, err := s.svc.GetResearchChat(r.PathValue("id"), r.PathValue("cid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": sess, "citations": cites})
}

func (s *Server) updateResearchChat(w http.ResponseWriter, r *http.Request) {
	var p service.ChatPatch
	if err := decode(w, r, &p); err != nil {
		s.fail(w, r, err)
		return
	}
	sess, err := s.svc.UpdateResearchChat(r.PathValue("id"), r.PathValue("cid"), p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) deleteResearchChat(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteResearchChat(r.PathValue("id"), r.PathValue("cid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sendResearch streams an answer: delta events, then finished (the stored
// turns with citations) or error.
func (s *Server) sendResearch(w http.ResponseWriter, r *http.Request) {
	var body struct{ Text string }
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	if _, _, err := s.svc.GetResearchChat(r.PathValue("id"), r.PathValue("cid")); err != nil {
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
	reply, err := s.svc.SendResearch(r.Context(), r.PathValue("id"), r.PathValue("cid"), body.Text,
		func(d string) { sse.send("delta", map[string]string{keyText: d}) })
	if err != nil {
		status, msg := classify(err)
		sse.send(keyError, map[string]any{keyStatus: status, keyError: msg, keyReply: reply})
		return
	}
	sse.send("finished", reply)
}

func (s *Server) noteFromMessage(w http.ResponseWriter, r *http.Request) {
	n, err := s.svc.NoteFromMessage(r.PathValue("id"), r.PathValue("cid"), r.PathValue("mid"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) listResearchNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := s.svc.ResearchNotes(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": notes})
}

// saveResearchNote creates a note (POST) or updates one (PATCH: only the
// fields given change).
func (s *Server) saveResearchNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title   *string `json:"title"`
		Text    *string `json:"text"`
		Include *bool   `json:"include"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	note := research.Note{Include: true}
	status := http.StatusCreated
	if nid := r.PathValue("nid"); nid != "" {
		notes, err := s.svc.ResearchNotes(r.PathValue("id"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		found := false
		for _, n := range notes {
			if n.ID == nid {
				note, found = n, true
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "note not found")
			return
		}
		status = http.StatusOK
	}
	if body.Title != nil {
		note.Title = *body.Title
	}
	if body.Text != nil {
		note.Text = *body.Text
	}
	if body.Include != nil {
		note.Include = *body.Include
	}
	saved, err := s.svc.SaveResearchNote(r.PathValue("id"), note)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, saved)
}

func (s *Server) deleteResearchNote(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteResearchNote(r.PathValue("id"), r.PathValue("nid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// JSON keys shared by the streaming handlers.
const (
	keyText  = "text"
	keyReply = "reply"
)
