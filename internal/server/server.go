// Package server exposes the service layer to the desktop app as a local
// JSON API (with server-sent events for runs) over a Unix socket or a
// loopback port. Every request needs the per-session bearer token.
package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/service"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// MaxConcurrentRuns bounds model runs in flight.
const MaxConcurrentRuns = 4

// Server is the HTTP API.
type Server struct {
	// Events, when set, backs GET /v1/events (scheduler notifications).
	Events *Broker

	svc     *service.Service
	token   string
	version string
	runs    chan struct{}
	log     *slog.Logger
}

// New returns a server for svc guarded by token.
func New(svc *service.Service, token, version string) *Server {
	return &Server{
		svc: svc, token: token, version: version, runs: make(chan struct{}, MaxConcurrentRuns), log: newLogger(),
	}
}

// keyError is the JSON field and SSE event name for errors.
const keyError = "error"

// Handler returns the routed, secured handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/worktypes", s.workTypes)
	mux.HandleFunc("GET /v1/notebooks", s.listNotebooks)
	mux.HandleFunc("POST /v1/notebooks", s.createNotebook)
	mux.HandleFunc("GET /v1/notebooks/{id}", s.getNotebook)
	mux.HandleFunc("POST /v1/notebooks/{id}/context", s.addContext)
	mux.HandleFunc("PATCH /v1/notebooks/{id}/context/{ctx}", s.setContext)
	mux.HandleFunc("POST /v1/notebooks/{id}/runs", s.startRun)
	mux.HandleFunc("GET /v1/notebooks/{id}/outputs/{out}", s.getOutput)
	mux.HandleFunc("GET /v1/outputs", s.listOutputs)
	mux.HandleFunc("GET /v1/settings", s.settings)
	mux.HandleFunc("GET /v1/models", s.models)
	mux.HandleFunc("GET /v1/events", s.events)
	mux.HandleFunc("GET /v1/templates", s.listTemplates)
	mux.HandleFunc("GET /v1/templates/{tid}", s.getTemplate)
	mux.HandleFunc("POST /v1/templates", s.createTemplate)
	mux.HandleFunc("DELETE /v1/templates/{tid}", s.deleteTemplate)
	mux.HandleFunc("PUT /v1/notebooks/{id}/memory", s.setMemory)
	mux.HandleFunc("GET /v1/notebooks/{id}/schedules", s.listSchedules)
	mux.HandleFunc("POST /v1/notebooks/{id}/schedules", s.addSchedule)
	mux.HandleFunc("PATCH /v1/notebooks/{id}/schedules/{sid}", s.setSchedule)
	mux.HandleFunc("DELETE /v1/notebooks/{id}/schedules/{sid}", s.deleteSchedule)
	mux.HandleFunc("GET /v1/chats", s.listChats)
	mux.HandleFunc("POST /v1/chats", s.createChat)
	mux.HandleFunc("GET /v1/chats/{cid}", s.getChat)
	mux.HandleFunc("PATCH /v1/chats/{cid}", s.updateChat)
	mux.HandleFunc("DELETE /v1/chats/{cid}", s.deleteChat)
	mux.HandleFunc("POST /v1/chats/{cid}/messages", s.sendChat)
	mux.HandleFunc("GET /v1/chats/{cid}/repeatable", s.planRepeatable)
	mux.HandleFunc("POST /v1/chats/{cid}/notebook", s.makeRepeatable)
	mux.HandleFunc("POST /v1/chats/{cid}/messages/{mid}/output", s.saveReply)
	mux.HandleFunc("GET /v1/chat-memory", s.chatMemory)
	mux.HandleFunc("PUT /v1/chat-memory", s.setChatMemory)
	return s.secure(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "version": s.version})
}

type workTypeJSON struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Question         string          `json:"question"`
	StarterPrompt    string          `json:"starterPrompt"`
	SuggestedContext []string        `json:"suggestedContext"`
	Schema           json.RawMessage `json:"schema"`
}

func (s *Server) workTypes(w http.ResponseWriter, _ *http.Request) {
	out := []workTypeJSON{}
	for _, wt := range worktype.All() {
		out = append(out, workTypeJSON{wt.ID, wt.Name, wt.Question, wt.StarterPrompt, wt.SuggestedContext, wt.Schema})
	}
	writeJSON(w, http.StatusOK, map[string]any{"workTypes": out})
}

func (s *Server) listNotebooks(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListNotebooks()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"notebooks": list})
}

func (s *Server) createNotebook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		WorkType   string `json:"workType"`
		TemplateID string `json:"templateId"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	var (
		info service.NotebookInfo
		err  error
	)
	if body.TemplateID != "" {
		info, err = s.svc.CreateFromTemplate(body.TemplateID, body.Name)
	} else {
		info, err = s.svc.CreateNotebook(body.Name, body.WorkType)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) getNotebook(w http.ResponseWriter, r *http.Request) {
	info, err := s.svc.Describe(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) addContext(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	info, err := s.svc.AddContext(r.PathValue("id"), body.Path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) setContext(w http.ResponseWriter, r *http.Request) {
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
	info, err := s.svc.SetContextEnabled(r.PathValue("id"), r.PathValue("ctx"), *body.Enabled)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

type runBody struct {
	Prompt     string             `json:"prompt"`
	PromptName string             `json:"promptName"`
	Vars       map[string]string  `json:"vars"`
	Overrides  config.RunSettings `json:"overrides"`
	Save       *bool              `json:"save"`
}

// startRun streams a run as server-sent events: status, delta, then exactly
// one of finished or error. Disconnecting cancels the run.
func (s *Server) startRun(w http.ResponseWriter, r *http.Request) {
	var body runBody
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	nb, err := s.svc.OpenNotebook(r.PathValue("id"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	select {
	case s.runs <- struct{}{}:
		defer func() { <-s.runs }()
	default:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d runs at once", MaxConcurrentRuns))
		return
	}
	// Runs outlive the server's read timeout: clear connection deadlines.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	sse, ok := newSSE(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	req := service.RunRequest{
		Notebook: nb, Prompt: body.Prompt, PromptName: body.PromptName, Vars: body.Vars,
		Overrides: body.Overrides, Save: body.Save == nil || *body.Save,
	}
	onEvent := func(e executor.Event) { sse.send(e.Kind, map[string]string{"text": e.Text}) }
	out, err := s.svc.Run(r.Context(), req, onEvent)
	if err != nil {
		status, msg := classify(err)
		sse.send(keyError, map[string]any{"status": status, keyError: msg})
		return
	}
	sse.send("finished", finishedEvent(out))
}

func finishedEvent(out service.RunOutcome) map[string]any {
	attempts := make([]map[string]string, 0, len(out.Result.Attempts))
	for _, a := range out.Result.Attempts {
		m := map[string]string{"target": a.Target, "trigger": a.Trigger}
		if a.Err != nil {
			m[keyError] = a.Err.Error()
		}
		attempts = append(attempts, m)
	}
	return map[string]any{
		"runId": out.RunID, "outputId": out.OutputID, "valid": out.Result.Valid, "modelUsed": out.Result.Model,
		"strategy": out.Result.Strategy, "totalTokens": out.Result.TotalTokens, "attempts": attempts,
		"data": out.Result.Data, "markdown": out.Markdown,
	}
}

func (s *Server) listOutputs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	outs, err := s.svc.ListOutputs(q.Get("notebook"), q.Get("workType"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"outputs": outs})
}

func (s *Server) getOutput(w http.ResponseWriter, r *http.Request) {
	o, err := s.svc.GetOutput(r.PathValue("id"), r.PathValue("out"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (s *Server) settings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.Settings())
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	ms, err := s.svc.Models(r.Context(), r.URL.Query().Get("provider"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": ms})
}
