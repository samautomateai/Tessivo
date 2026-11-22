package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestURLAndTextSourcesAreSentToTheModel(t *testing.T) {
	page := "Ship in March"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<p>" + page + "</p>"))
	}))
	defer srv.Close()
	s := newSvc(t, summary)
	info, _ := s.CreateNotebook("Ops", "summarize")
	if _, err := s.AddURLContext(info.ID, srv.URL+"/status", "Status page"); err != nil {
		t.Fatal(err)
	}
	got, err := s.AddTextContext(info.ID, "Notes", "Budget is 10k.")
	if err != nil || len(got.Context) != 2 || got.Context[0].Kind != "url" {
		t.Fatalf("%v %+v", err, got.Context)
	}
	nb, _ := s.OpenNotebook(info.ID)
	docs, err := s.notebookDocs(context.Background(), nb)
	if err != nil {
		t.Fatal(err)
	}
	all := ""
	for _, d := range docs {
		all += d.Text + "\n"
	}
	if !strings.Contains(all, "Ship in March") || !strings.Contains(all, "Budget is 10k.") {
		t.Fatalf("docs: %q", all)
	}
	page = "Ship in April" // read on every run
	docs, _ = s.notebookDocs(context.Background(), nb)
	if !strings.Contains(docs[0].Text, "April") {
		t.Fatalf("not re-read: %q", docs[0].Text)
	}
	if _, err := s.AddURLContext(info.ID, "file:///etc/passwd", ""); !IsInvalid(err) {
		t.Fatalf("file URL: %v", err)
	}
	if _, err := s.AddTextContext(info.ID, "x", " "); !IsInvalid(err) {
		t.Fatalf("empty text: %v", err)
	}
}
