package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/service"
)

func TestUsageAPIAndLimitStatus(t *testing.T) {
	ts, s := newTestServer(t, summary)
	s.svc.Config.Limits.MaxTokensPerDay = 5000
	code, body := call(t, ts, "GET", "/v1/usage", "")
	if code != 200 || !strings.Contains(body, `"dayLimit":5000`) || !strings.Contains(body, `"tokens":0`) {
		t.Fatalf("usage: %d %s", code, body)
	}
	if status, _ := classify(service.ErrTokenLimit); status != http.StatusTooManyRequests {
		t.Fatalf("token limit maps to %d", status)
	}
}
