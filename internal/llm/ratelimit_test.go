package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
)

func TestRetryAfterIsReadFromHeadersAndGeminiBodies(t *testing.T) {
	for _, tc := range []struct {
		header, value, body string
		want                time.Duration
	}{
		{"Retry-After", "7", "", 7 * time.Second},
		{"retry-after-ms", "1500", "", 1500 * time.Millisecond},
		{
			"", "", `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"12s"}]}}`,
			12 * time.Second,
		},
		{"", "", `{"error":"busy"}`, 0},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if tc.header != "" {
				w.Header().Set(tc.header, tc.value)
			}
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := NewClient(srv.URL, "k", time.Minute).Chat(context.Background(), Request{Model: "m"})
		srv.Close()
		var api *APIError
		if !errors.As(err, &api) || api.RetryAfter != tc.want {
			t.Errorf("%s %s: got %v", tc.header, tc.body, err)
		}
	}
}

// scripted answers each call with the next error (nil: success).
type scripted struct {
	mu    sync.Mutex
	errs  []error
	calls int
}

func (s *scripted) next() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if len(s.errs) == 0 {
		return nil
	}
	e := s.errs[0]
	s.errs = s.errs[1:]
	return e
}

func (s *scripted) Chat(context.Context, Request) (Response, error) {
	return Response{Content: "ok"}, s.next()
}

func (s *scripted) ChatStream(_ context.Context, _ Request, _ func(string)) (Response, error) {
	return Response{Content: "ok"}, s.next()
}

func TestRouterHonoursRetryAfterAndFallsBackOnLongWaits(t *testing.T) {
	var waited []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { waited = append(waited, d); return nil }

	a := &scripted{errs: []error{&APIError{Status: 429, RetryAfter: 3 * time.Second}}}
	r := &Router{
		Chain: []Target{{Label: "p/a", Client: a}}, Triggers: config.DefaultTriggers, MaxRetries: 2,
		sleep: sleep,
	}
	if _, err := r.Chat(context.Background(), Request{}); err != nil || a.calls != 2 {
		t.Fatalf("retry after the provider's wait: %v, %d calls", err, a.calls)
	}
	if len(waited) != 1 || waited[0] < 3*time.Second || waited[0] > 4*time.Second {
		t.Fatalf("waited %v, want about the 3 s the provider asked for", waited)
	}

	waited = nil
	slow := &scripted{errs: []error{&APIError{Status: 429, RetryAfter: 5 * time.Minute}}}
	next := &scripted{}
	r = &Router{
		Chain:    []Target{{Label: "p/slow", Client: slow}, {Label: "p/next", Client: next}},
		Triggers: config.DefaultTriggers, MaxRetries: 2, sleep: sleep,
	}
	res, err := r.Chat(context.Background(), Request{})
	if err != nil || res.Target != "p/next" || len(waited) != 0 || slow.calls != 1 {
		t.Fatalf("a long wait must fall back at once: %v %s waited %v calls %d", err, res.Target, waited, slow.calls)
	}

	alone := &scripted{errs: []error{&APIError{Status: 429, RetryAfter: 5 * time.Minute}}}
	r = &Router{
		Chain: []Target{{Label: "p/alone", Client: alone}}, Triggers: config.DefaultTriggers, MaxRetries: 2,
		sleep: sleep,
	}
	if _, err := r.Chat(context.Background(), Request{}); err == nil || !strings.Contains(err.Error(), "retry in 5m") {
		t.Fatalf("a lone model must say when to retry: %v", err)
	}
}

func TestBackoffHasJitter(t *testing.T) {
	seen := map[time.Duration]bool{}
	for range 20 {
		seen[jittered(time.Second)] = true
	}
	if len(seen) < 2 {
		t.Fatal("no jitter")
	}
	for d := range seen {
		if d < time.Second || d > 1250*time.Millisecond {
			t.Fatalf("jitter out of range: %v", d)
		}
	}
}

func TestContextOverflowWordingOfEveryProvider(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"This model's maximum context length is 8192 tokens"}}`,
		`{"type":"error","error":{"message":"prompt is too long: 210000 tokens > 200000 maximum"}}`,
		`{"error":{"code":400,"message":"The input token count (1200000) exceeds the maximum number of tokens"}}`,
	} {
		if got := Classify(&APIError{Status: 400, Body: body}); got != config.TriggerContextOverflow {
			t.Errorf("%s: %q", body, got)
		}
	}
}

func TestErrorMessagesSayWhatToDo(t *testing.T) {
	for status, want := range map[int]string{
		401: "check the API key", 403: "access denied", 404: "not found",
		429: "rate limited",
	} {
		if msg := (&APIError{Status: status, Body: "x"}).Error(); !strings.Contains(msg, want) {
			t.Errorf("%d: %q", status, msg)
		}
	}
}

func TestProviderConcurrencyCap(t *testing.T) {
	var now, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := now.Add(1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		time.Sleep(30 * time.Millisecond)
		now.Add(-1)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()
	c := NewProviderClient(config.Provider{Name: "local-cap-test", BaseURL: srv.URL, MaxConcurrent: 2}, "")
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Chat(context.Background(), Request{Model: "m"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if peak.Load() > 2 {
		t.Fatalf("%d requests ran at once; the cap is 2", peak.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Chat(ctx, Request{Model: "m"}); err == nil {
		t.Fatal("a cancelled request waited for a slot")
	}
}
