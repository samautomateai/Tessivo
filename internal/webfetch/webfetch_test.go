package webfetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchHTMLBecomesText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credentials were sent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Status &amp; plans</title><style>p{color:red}</style>
<script>alert("x")</script></head><body><h1>Q3</h1><p>Ship in <b>March</b>.</p>
<ul><li>one</li><li>two</li></ul></body></html>`))
	}))
	defer srv.Close()
	d, err := Fetch(context.Background(), srv.URL+"/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Status & plans", "Q3", "Ship in March.", "one", "two"} {
		if !strings.Contains(d.Text, want) {
			t.Errorf("missing %q in %q", want, d.Text)
		}
	}
	if strings.Contains(d.Text, "alert") || strings.Contains(d.Text, "color:red") || strings.Contains(d.Text, "<") {
		t.Fatalf("script, style or tags left in: %q", d.Text)
	}
	if d.Source != srv.URL+"/status" {
		t.Fatalf("source %q", d.Source)
	}
}

func TestFetchRefusals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/big":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(strings.Repeat("x", MaxBytes+10)))
		case "/image":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/to-file":
			w.Header().Set("Location", "file:///etc/passwd")
			w.WriteHeader(http.StatusFound)
		case "/missing":
			http.NotFound(w, r)
		default:
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()
	for _, u := range []string{
		srv.URL + "/big", srv.URL + "/image", srv.URL + "/loop", srv.URL + "/to-file", srv.URL + "/missing",
		"ftp://example.com/x", "file:///etc/passwd", "http://user:pass@example.com/", "not a url", "",
		"http://" + strings.Repeat("a", MaxURL) + ".com/",
	} {
		if _, err := Fetch(context.Background(), u); err == nil {
			t.Errorf("%.60s: fetched", u)
		}
	}
	if err := Check("http://user:pass@example.com/"); !errors.Is(err, ErrBadURL) {
		t.Fatalf("credentials in URL: %v", err)
	}
	if d, err := Fetch(context.Background(), srv.URL+"/plain"); err != nil || d.Text != "ok" {
		t.Fatalf("%v %+v", err, d)
	}
}

func FuzzHTMLText(f *testing.F) {
	for _, s := range []string{"<p>a</p>", "<script>x", "&amp;&#x41;", "<<>>", "<!-- c -->t", "<style>a{}</style>b"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := HTMLText(s)
		if len(out) > len(s)+len(s)/2+16 {
			t.Fatalf("output grew from %d to %d bytes", len(s), len(out))
		}
	})
}
