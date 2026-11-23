package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/config"
)

// APIError is a non-2xx response from a provider.
type APIError struct {
	Status int
	Body   string
	// RetryAfter is how long the provider asked callers to wait (0: it did not say).
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	hint := ""
	switch {
	case e.Status == 401:
		hint = " (check the API key)"
	case e.Status == 403:
		hint = " (access denied: the key may lack access to this model)"
	case e.Status == 404:
		hint = " (model or URL not found)"
	case e.Status == 429 && e.RetryAfter > 0:
		hint = fmt.Sprintf(" (rate limited; retry in %s)", e.RetryAfter.Round(time.Second))
	case e.Status == 429:
		hint = " (rate limited)"
	}
	return fmt.Sprintf("provider returned HTTP %d%s: %s", e.Status, hint, strings.TrimSpace(body))
}

// TransportError wraps network failures (DNS, connection, timeouts).
type TransportError struct{ Err error }

func (e *TransportError) Error() string { return "transport: " + e.Err.Error() }

// Unwrap exposes the underlying error.
func (e *TransportError) Unwrap() error { return e.Err }

// ErrInvalidOutput marks output that failed schema validation after retries.
var ErrInvalidOutput = errors.New("output failed schema validation")

// Classify maps an error to a fallback trigger name, or "" if none applies.
func Classify(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrInvalidOutput) {
		return config.TriggerInvalidOutput
	}
	var api *APIError
	if errors.As(err, &api) {
		return classifyAPI(api)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return config.TriggerTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return config.TriggerTimeout
	}
	var te *TransportError
	if errors.As(err, &te) {
		return config.TriggerServerError
	}
	return ""
}

func classifyAPI(e *APIError) string {
	body := strings.ToLower(e.Body)
	switch {
	case e.Status == 429:
		return config.TriggerRateLimit
	case e.Status == 408 || e.Status == 504:
		return config.TriggerTimeout
	case e.Status >= 500:
		return config.TriggerServerError
	case (e.Status == 400 || e.Status == 413) && overflowed(body):
		return config.TriggerContextOverflow
	default:
		return ""
	}
}

// retryable reports whether retrying the same model may help.
func retryable(trigger string) bool {
	return trigger == config.TriggerRateLimit || trigger == config.TriggerServerError ||
		trigger == config.TriggerTimeout
}

// overflowWords are how providers say a request is too large for the model.
var overflowWords = []string{
	"context_length", "maximum context", "context window", // OpenAI-compatible
	"prompt is too long",                                        // Anthropic
	"exceeds the maximum number of tokens", "input token count", // Gemini
}

func overflowed(body string) bool {
	for _, w := range overflowWords {
		if strings.Contains(body, w) {
			return true
		}
	}
	return false
}

var retryDelay = regexp.MustCompile(`"retryDelay"\s*:\s*"(\d+(?:\.\d+)?)s"`)

// retryAfter reads how long a provider asked to wait: Retry-After (seconds or
// an HTTP date), retry-after-ms, or Gemini's RetryInfo retryDelay in the body.
func retryAfter(h http.Header, body string, now time.Time) time.Duration {
	if ms, err := strconv.Atoi(h.Get("retry-after-ms")); err == nil && ms > 0 {
		return time.Duration(ms) * time.Millisecond
	}
	if v := h.Get("Retry-After"); v != "" {
		if s, err := strconv.Atoi(v); err == nil && s > 0 {
			return time.Duration(s) * time.Second
		}
		if t, err := http.ParseTime(v); err == nil && t.After(now) {
			return t.Sub(now)
		}
	}
	if m := retryDelay.FindStringSubmatch(body); m != nil {
		if s, err := strconv.ParseFloat(m[1], 64); err == nil && s > 0 {
			return time.Duration(s * float64(time.Second))
		}
	}
	return 0
}
