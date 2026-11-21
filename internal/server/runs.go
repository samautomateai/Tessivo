package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/samautomateai/tessivo/internal/secrets"
)

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.Pending()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": list})
}

type answerBody struct {
	Decision string `json:"decision"`
	Remember string `json:"remember"`
}

// answerApproval records the answer and resumes the run in the background.
func (s *Server) answerApproval(w http.ResponseWriter, r *http.Request) {
	var body answerBody
	if err := decode(w, r, &body); err != nil {
		s.fail(w, r, err)
		return
	}
	runID := r.PathValue("rid")
	if err := s.svc.Answer(runID, r.PathValue("aid"), body.Decision, body.Remember); err != nil {
		s.fail(w, r, err)
		return
	}
	s.startResume(r.Context(), w, runID)
}

func (s *Server) resumeRun(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("rid")
	if err := s.svc.ResumeCheck(runID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.startResume(r.Context(), w, runID)
}

// startResume continues the run detached from the request when it is ready.
// A run still waiting on another approval answers 202 without resuming.
func (s *Server) startResume(reqCtx context.Context, w http.ResponseWriter, runID string) {
	if err := s.svc.ResumeCheck(runID); err != nil {
		writeJSON(w, http.StatusAccepted, map[string]string{keyStatus: "answered"})
		return
	}
	select {
	case s.runs <- struct{}{}:
	default:
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("at most %d runs at once", MaxConcurrentRuns))
		return
	}
	// The run outlives the request (it is the core's job) but stops with the server.
	ctx, cancel := context.WithCancel(context.WithoutCancel(reqCtx))
	stopOnShutdown := func() bool { return true }
	if s.Background != nil {
		//nolint:contextcheck // deliberate: watches the server's shutdown context, not the request's
		stopOnShutdown = context.AfterFunc(s.Background, cancel)
	}
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		defer cancel()
		defer stopOnShutdown()
		defer func() { <-s.runs }()
		if _, err := s.svc.Resume(ctx, runID, nil); err != nil {
			s.log.Info("resumed run ended with an error", "run", runID, "err", secrets.RedactPatterns(err.Error()))
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{keyStatus: "resuming"})
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Cancel(r.PathValue("rid")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// runEvents streams a live run's events, or sends one status event for a run
// not executing in this process.
func (s *Server) runEvents(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("rid")
	ch, stop, live := s.svc.Subscribe(runID)
	defer stop()
	var status string
	if !live {
		var err error
		if status, err = s.svc.RunStatus(runID); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	sse, ok := newSSE(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	if !live {
		sse.send(keyStatus, map[string]string{keyRunID: runID, keyStatus: status})
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, open := <-ch:
			if !open {
				// The run ended (or was ending when we attached): always close
				// with its stored status, so a client never sees an empty stream.
				if st, err := s.svc.RunStatus(runID); err == nil {
					sse.send(keyStatus, map[string]string{keyRunID: runID, keyStatus: st})
				}
				return
			}
			sse.send(e.Kind, eventPayload(e))
		}
	}
}

// Wait blocks until runs resumed in the background have stopped.
func (s *Server) Wait() { s.bg.Wait() }
