package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
)

// fake answers with a scripted sequence of errors, then succeeds.
type fake struct {
	errs     []error
	calls    int
	partial  bool // ChatStream emits a delta before failing
	response string
}

func (f *fake) next() error {
	f.calls++
	if f.calls <= len(f.errs) {
		return f.errs[f.calls-1]
	}
	return nil
}

func (f *fake) Chat(_ context.Context, req Request) (Response, error) {
	if err := f.next(); err != nil {
		return Response{}, err
	}
	return Response{Model: req.Model, Content: f.response}, nil
}

func (f *fake) ChatStream(_ context.Context, req Request, onDelta func(string)) (Response, error) {
	err := f.next()
	if err != nil && f.partial {
		onDelta("par")
		return Response{}, err
	}
	if err != nil {
		return Response{}, err
	}
	onDelta(f.response)
	return Response{Model: req.Model, Content: f.response}, nil
}

func noSleep(context.Context, time.Duration) error { return nil }

func TestRouterRetriesThenSucceeds(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 429}, &APIError{Status: 503}}, response: "ok"}
	r := &Router{Chain: []Target{{"p/a", "a", a}}, Triggers: config.DefaultTriggers, MaxRetries: 2, sleep: noSleep}
	res, err := r.Chat(context.Background(), Request{})
	if err != nil || res.Content != "ok" || res.Target != "p/a" || len(res.Attempts) != 3 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestRouterFallsBack(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 429}, &APIError{Status: 429}}}
	b := &fake{response: "from b"}
	r := &Router{
		Chain: []Target{{"p/a", "a", a}, {"p/b", "b", b}}, Triggers: config.DefaultTriggers, MaxRetries: 1,
		sleep: noSleep,
	}
	res, err := r.Chat(context.Background(), Request{})
	if err != nil || res.Target != "p/b" || res.Model != "b" {
		t.Fatalf("res %+v err %v", res, err)
	}
	if res.Attempts[0].Trigger != config.TriggerRateLimit {
		t.Fatalf("attempts %+v", res.Attempts)
	}
}

func TestRouterStopsWhenTriggerDisabled(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 500}}}
	b := &fake{response: "b"}
	r := &Router{
		Chain: []Target{{"p/a", "a", a}, {"p/b", "b", b}}, Triggers: []string{config.TriggerRateLimit},
		sleep: noSleep,
	}
	if _, err := r.Chat(context.Background(), Request{}); err == nil || b.calls != 0 {
		t.Fatalf("want stop without fallback, err %v calls %d", err, b.calls)
	}
}

func TestRouterNonRetryableFallsBackImmediately(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 400, Body: "context_length_exceeded"}}}
	b := &fake{response: "b"}
	r := &Router{
		Chain: []Target{{"p/a", "a", a}, {"p/b", "b", b}}, Triggers: config.DefaultTriggers, MaxRetries: 3,
		sleep: noSleep,
	}
	res, err := r.Chat(context.Background(), Request{})
	if err != nil || a.calls != 1 || res.Target != "p/b" {
		t.Fatalf("res %+v err %v calls %d", res, err, a.calls)
	}
}

func TestRouterLastTargetFails(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 401}}}
	r := &Router{Chain: []Target{{"p/a", "a", a}}, Triggers: config.DefaultTriggers}
	if _, err := r.Chat(context.Background(), Request{}); err == nil {
		t.Fatal("want error")
	}
	if _, err := (&Router{}).Chat(context.Background(), Request{}); err == nil {
		t.Fatal("want empty chain error")
	}
}

func TestRouterStreamNoFallbackAfterOutput(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 503}}, partial: true}
	b := &fake{response: "b"}
	r := &Router{
		Chain: []Target{{"p/a", "a", a}, {"p/b", "b", b}}, Triggers: config.DefaultTriggers, MaxRetries: 2,
		sleep: noSleep,
	}
	var got string
	_, err := r.ChatStream(context.Background(), Request{}, func(s string) { got += s })
	if err == nil || b.calls != 0 || a.calls != 1 || got != "par" {
		t.Fatalf("err %v b.calls %d a.calls %d got %q", err, b.calls, a.calls, got)
	}
}

func TestRouterStreamFallsBackBeforeOutput(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 503}}}
	b := &fake{response: "b"}
	r := &Router{Chain: []Target{{"p/a", "a", a}, {"p/b", "b", b}}, Triggers: config.DefaultTriggers, sleep: noSleep}
	res, err := r.ChatStream(context.Background(), Request{}, nil)
	if err != nil || res.Target != "p/b" {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestRouterCancelledDuringBackoff(t *testing.T) {
	a := &fake{errs: []error{&APIError{Status: 503}, &APIError{Status: 503}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &Router{
		Chain: []Target{{"p/a", "a", a}, {"p/b", "b", &fake{}}}, Triggers: config.DefaultTriggers,
		MaxRetries: 3, Backoff: func(int) time.Duration { return time.Hour },
	}
	_, err := r.Chat(ctx, Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
}

func TestDefaultBackoff(t *testing.T) {
	if DefaultBackoff(0) != 500*time.Millisecond || DefaultBackoff(1) != time.Second ||
		DefaultBackoff(10) != 8*time.Second {
		t.Fatal("backoff schedule")
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
}
