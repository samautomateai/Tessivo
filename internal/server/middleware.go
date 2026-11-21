package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

const localhost = "localhost"

// MaxBodyBytes caps request bodies.
const MaxBodyBytes = 1 << 20

// secure wraps every route: panic recovery, security headers, browser-origin
// refusal (stops websites reaching the loopback port), Host checks (stops DNS
// rebinding) and constant-time bearer-token auth.
func (s *Server) secure(next http.Handler) http.Handler {
	want := []byte("Bearer " + s.token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic in handler", "path", r.URL.Path, "panic", fmt.Sprint(v))
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'")
		if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Site") != "" {
			writeError(w, http.StatusForbidden, "browser requests are not allowed")
			return
		}
		if !allowedHost(r.Host) {
			writeError(w, http.StatusMisdirectedRequest, "unexpected host")
			return
		}
		if r.URL.Path == "/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost accepts loopback hosts and the placeholder Host used over Unix
// sockets; anything else suggests DNS rebinding.
func allowedHost(host string) bool {
	if host == "" || host == "tessivo" || host == localhost {
		return true
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	if h == localhost {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// decode reads a JSON body strictly into v.
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return &service.InvalidError{Msg: "Content-Type must be application/json"}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &service.InvalidError{Msg: "invalid JSON body: " + err.Error()}
	}
	if dec.More() {
		return &service.InvalidError{Msg: "invalid JSON body: trailing data"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{keyError: msg})
}

// fail maps service errors to HTTP statuses without leaking secrets.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := classify(err)
	if status >= 500 {
		s.log.Error("request failed", "path", r.URL.Path, "err", secrets.RedactPatterns(err.Error()))
	}
	writeError(w, status, msg)
}

func classify(err error) (int, string) {
	msg := secrets.RedactPatterns(err.Error())
	switch {
	case service.IsInvalid(err):
		return http.StatusBadRequest, msg
	case errors.Is(err, service.ErrNotFound):
		return http.StatusNotFound, msg
	case errors.Is(err, service.ErrConflict), errors.Is(err, notebook.ErrRunLocked):
		return http.StatusConflict, msg
	case errors.Is(err, llm.ErrInvalidOutput):
		return http.StatusUnprocessableEntity, msg
	default:
		return http.StatusInternalServerError, msg
	}
}

func newLogger() *slog.Logger { return slog.Default() }
