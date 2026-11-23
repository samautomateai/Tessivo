package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/samautomateai/tessivo/internal/service"
)

// chatApprovalWait is how long a chat reply waits for the user's answer to a tool call.
const chatApprovalWait = 5 * time.Minute

// chatApprovals holds the tool calls chat replies are waiting on, by chat and call ID.
type chatApprovals struct {
	mu      sync.Mutex
	waiting map[string]chan service.ChatToolAnswer
}

// wait blocks until key is answered, ctx ends or timeout passes (then: denied).
func (a *chatApprovals) wait(ctx context.Context, key string, timeout time.Duration) service.ChatToolAnswer {
	ch := make(chan service.ChatToolAnswer, 1)
	a.mu.Lock()
	if a.waiting == nil {
		a.waiting = map[string]chan service.ChatToolAnswer{}
	}
	a.waiting[key] = ch
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.waiting, key)
		a.mu.Unlock()
	}()
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case ans := <-ch:
		return ans
	case <-ctx.Done():
	case <-t.C:
	}
	return service.ChatToolAnswer{}
}

// answer delivers ans to the reply waiting on key; false when none waits.
func (a *chatApprovals) answer(key string, ans service.ChatToolAnswer) bool {
	a.mu.Lock()
	ch, ok := a.waiting[key]
	a.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- ans:
		return true
	default:
		return false // already answered
	}
}

func (s *Server) answerChatTool(w http.ResponseWriter, r *http.Request) {
	var ans service.ChatToolAnswer
	if err := decode(w, r, &ans); err != nil {
		s.fail(w, r, err)
		return
	}
	if !s.chatAsk.answer(r.PathValue("cid")+"/"+r.PathValue("aid"), ans) {
		writeError(w, http.StatusNotFound, "no tool call is waiting for that answer")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
