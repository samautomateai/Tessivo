package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"time"
)

// Target is one model in a fallback chain.
type Target struct {
	Label  string // e.g. "litellm/model-a", for logs and run records
	Model  string // model name sent to the provider
	Client Chatter
}

// Attempt records one try against one target.
type Attempt struct {
	Target  string
	Trigger string // why it failed ("" if it succeeded or failed without a trigger)
	Err     error
}

// Result is a routed response plus what happened on the way.
type Result struct {
	Response
	Target   string // label of the model that answered
	Attempts []Attempt
}

// Router sends a request down an ordered chain of targets, retrying the same
// target with backoff on transient errors and falling back to the next target
// when a configured trigger fires.
type Router struct {
	Chain      []Target
	Triggers   []string
	MaxRetries int
	Backoff    func(attempt int) time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
}

// DefaultBackoff is exponential: 0.5s, 1s, 2s … capped at 8s.
func DefaultBackoff(attempt int) time.Duration {
	d := 500 * time.Millisecond << attempt
	return min(d, 8*time.Second)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait before retry: %w", ctx.Err())
	case <-t.C:
		return nil
	}
}

// Chat routes a non-streaming request.
func (r *Router) Chat(ctx context.Context, req Request) (Result, error) {
	return r.route(ctx, req, func(t Target, rq Request) (Response, error) {
		return t.Client.Chat(ctx, rq)
	})
}

// ChatStream routes a streaming request. Fallback happens only before the
// first delta is delivered, so callers never see output from two models.
func (r *Router) ChatStream(ctx context.Context, req Request, onDelta func(string)) (Result, error) {
	return r.route(ctx, req, func(t Target, rq Request) (Response, error) {
		started := false
		resp, err := t.Client.ChatStream(ctx, rq, func(s string) {
			started = true
			if onDelta != nil {
				onDelta(s)
			}
		})
		if err != nil && started {
			return resp, &midStreamError{err}
		}
		return resp, err
	})
}

type midStreamError struct{ err error }

func (e *midStreamError) Error() string { return "stream interrupted: " + e.err.Error() }
func (e *midStreamError) Unwrap() error { return e.err }

func (r *Router) route(ctx context.Context, req Request, call func(Target, Request) (Response, error)) (Result, error) {
	if len(r.Chain) == 0 {
		return Result{}, errors.New("router: empty model chain")
	}
	backoff, sleep := r.Backoff, r.sleep
	if backoff == nil {
		backoff = DefaultBackoff
	}
	if sleep == nil {
		sleep = sleepCtx
	}
	var res Result
	for i, t := range r.Chain {
		rq := req
		rq.Model = t.Model
		resp, trigger, err := r.tryTarget(ctx, t, rq, call, backoff, sleep, &res)
		if err == nil {
			res.Response, res.Target = resp, t.Label
			return res, nil
		}
		var mid *midStreamError
		last := i == len(r.Chain)-1
		if last || errors.As(err, &mid) || ctx.Err() != nil || !slices.Contains(r.Triggers, trigger) {
			return res, fmt.Errorf("model %s: %w", t.Label, err)
		}
	}
	return res, errors.New("router: chain exhausted") // unreachable: the last target always returns above
}

func (r *Router) tryTarget(ctx context.Context, t Target, rq Request, call func(Target, Request) (Response, error),
	backoff func(int) time.Duration, sleep func(context.Context, time.Duration) error, res *Result,
) (Response, string, error) {
	for attempt := 0; ; attempt++ {
		resp, err := call(t, rq)
		trigger := Classify(err)
		res.Attempts = append(res.Attempts, Attempt{Target: t.Label, Trigger: trigger, Err: err})
		if err == nil {
			return resp, "", nil
		}
		var mid *midStreamError
		if errors.As(err, &mid) || !retryable(trigger) || attempt >= r.MaxRetries {
			return resp, trigger, err
		}
		wait := jittered(backoff(attempt))
		var api *APIError
		if errors.As(err, &api) && api.RetryAfter > 0 {
			if api.RetryAfter > MaxRetryWait {
				return resp, trigger, err // too long to wait here: fall back now (the error says when to retry)
			}
			wait = jittered(api.RetryAfter)
		}
		if serr := sleep(ctx, wait); serr != nil {
			return resp, trigger, serr
		}
	}
}

// MaxRetryWait is the longest a model is waited for before a retry; a
// provider asking for longer makes the router fall back (or fail) at once.
const MaxRetryWait = 30 * time.Second

// jittered adds up to 25% to d so parallel callers do not retry in step.
func jittered(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + rand.N(d/4+1) //nolint:gosec // G404: timing jitter, not security
}
