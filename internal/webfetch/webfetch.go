// Package webfetch reads a web page the user added as a source: http(s)
// only, no credentials in the URL, bounded in size, time and redirects, and
// text-like content only. HTML is reduced to its text.
package webfetch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/samautomateai/tessivo/internal/extract"
)

// Limits on a fetch.
const (
	MaxBytes     = 2 << 20 // body read, before HTML is reduced
	MaxURL       = 2048
	maxRedirects = 3
	timeout      = 20 * time.Second
)

// ErrBadURL means a URL is not an http(s) address Tessivo will fetch.
var ErrBadURL = errors.New("not an http or https URL without credentials")

// Check reports whether raw is a URL Fetch accepts.
func Check(raw string) error {
	if raw == "" || len(raw) > MaxURL {
		return fmt.Errorf("%w (1 to %d characters)", ErrBadURL, MaxURL)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return fmt.Errorf("%w: %.80s", ErrBadURL, raw)
	}
	return nil
}

var client = &http.Client{
	Timeout: timeout,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          4,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("more than %d redirects", maxRedirects)
		}
		return Check(req.URL.String())
	},
}

// Fetch reads the page at raw and returns its text.
func Fetch(ctx context.Context, raw string) (extract.Document, error) {
	if err := Check(raw); err != nil {
		return extract.Document{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return extract.Document{}, fmt.Errorf("fetch %s: %w", raw, err)
	}
	req.Header.Set("User-Agent", "Tessivo (+https://github.com/samautomateai/tessivo)")
	req.Header.Set("Accept", "text/html, text/markdown, text/plain, application/json;q=0.9, */*;q=0.1")
	resp, err := client.Do(req)
	if err != nil {
		return extract.Document{}, fmt.Errorf("fetch %s: %w", raw, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return extract.Document{}, fmt.Errorf("fetch %s: %s", raw, resp.Status)
	}
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if !textLike(mt) {
		return extract.Document{}, fmt.Errorf("fetch %s: %q is not text", raw, mt)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return extract.Document{}, fmt.Errorf("fetch %s: %w", raw, err)
	}
	if len(b) > MaxBytes {
		return extract.Document{}, fmt.Errorf("fetch %s: larger than %d bytes", raw, MaxBytes)
	}
	text := string(b)
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}
	kind := "text"
	switch mt {
	case "text/html", "application/xhtml+xml":
		text = HTMLText(text)
	case "text/markdown":
		kind = "markdown"
	}
	return extract.Document{Source: raw, Kind: kind, Text: strings.TrimSpace(text)}, nil
}

func textLike(mt string) bool {
	switch {
	case strings.HasPrefix(mt, "text/"), mt == "application/json", mt == "application/xml",
		mt == "application/xhtml+xml", strings.HasSuffix(mt, "+json"), strings.HasSuffix(mt, "+xml"):
		return true
	}
	return false
}
