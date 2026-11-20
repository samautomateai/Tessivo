package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// MaxSubscribers bounds concurrent /v1/events streams.
const MaxSubscribers = 16

// Broker fans events out to subscribers. Slow subscribers drop events
// rather than blocking publishers.
type Broker struct {
	mu   sync.Mutex
	subs map[chan any]struct{}
}

// NewBroker returns an empty broker.
func NewBroker() *Broker { return &Broker{subs: map[chan any]struct{}{}} }

// Publish delivers v to every subscriber that has room.
func (b *Broker) Publish(v any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- v:
		default: // subscriber is behind: drop
		}
	}
}

// Subscribe registers a subscriber; it fails when the broker is full.
func (b *Broker) Subscribe() (<-chan any, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.subs) >= MaxSubscribers {
		return nil, nil, fmt.Errorf("too many event subscribers")
	}
	ch := make(chan any, 32)
	b.subs[ch] = struct{}{}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subs, ch)
	}, nil
}

// keepAlive is how often an idle event stream sends a comment.
var keepAlive = 25 * time.Second

// events streams scheduler (and future) events as SSE until the client leaves.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if s.Events == nil {
		writeError(w, http.StatusNotFound, "events are not enabled")
		return
	}
	ch, cancel, err := s.Events.Subscribe()
	if err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	defer cancel()
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})
	sse, ok := newSSE(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	t := time.NewTicker(keepAlive)
	defer t.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case v := <-ch:
			sse.send("event", v)
		case <-t.C:
			sse.comment("keepalive")
		}
	}
}
