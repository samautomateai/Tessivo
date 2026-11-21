package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/samautomateai/tessivo/internal/config"
)

// APIError is a non-2xx response from a provider.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 300 {
		body = body[:300] + "…"
	}
	return fmt.Sprintf("provider returned HTTP %d: %s", e.Status, strings.TrimSpace(body))
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
	case e.Status == 400 && (strings.Contains(body, "context_length") ||
		strings.Contains(body, "maximum context") || strings.Contains(body, "context window")):
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
