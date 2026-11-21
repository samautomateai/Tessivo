package notify

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Dispatcher queues notes for one background worker so callers never block
// on an OS tool, and rate-limits them: beyond Burst notes per Window the rest
// are counted and summarized in the next note that goes out.
type Dispatcher struct {
	N      Notifier
	Burst  int           // default 5
	Window time.Duration // default 1 minute
	OnErr  func(error)   // optional; errors are otherwise dropped

	once    sync.Once
	queue   chan Note
	mu      sync.Mutex
	sent    []time.Time
	dropped int
	now     func() time.Time
}

// queueSize bounds notes waiting for the worker; more are dropped.
const queueSize = 16

func (d *Dispatcher) init() {
	d.once.Do(func() {
		d.queue = make(chan Note, queueSize)
		if d.Burst <= 0 {
			d.Burst = 5
		}
		if d.Window <= 0 {
			d.Window = time.Minute
		}
		if d.now == nil {
			d.now = time.Now
		}
	})
}

// Send queues n without blocking; it returns false when the queue is full.
func (d *Dispatcher) Send(n Note) bool {
	d.init()
	select {
	case d.queue <- n:
		return true
	default:
		d.mu.Lock()
		d.dropped++
		d.mu.Unlock()
		return false
	}
}

// Run delivers queued notes until ctx ends.
func (d *Dispatcher) Run(ctx context.Context) {
	d.init()
	for {
		select {
		case <-ctx.Done():
			return
		case n := <-d.queue:
			if n, ok := d.admit(n); ok {
				if err := d.N.Notify(ctx, n); err != nil && d.OnErr != nil {
					d.OnErr(err)
				}
			}
		}
	}
}

// admit applies the rate limit and folds earlier drops into n.
func (d *Dispatcher) admit(n Note) (Note, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	keep := d.sent[:0]
	for _, t := range d.sent {
		if now.Sub(t) < d.Window {
			keep = append(keep, t)
		}
	}
	d.sent = keep
	if len(d.sent) >= d.Burst {
		d.dropped++
		return Note{}, false
	}
	if d.dropped > 0 {
		n.Body = fmt.Sprintf("%s (+%d more)", n.Body, d.dropped)
		d.dropped = 0
	}
	d.sent = append(d.sent, now)
	return n, true
}
