package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

const apiAnalysis = `{"title":"Review","summary":"s","risk_score":7,"risk_level":"high","findings":[{"title":"A",` +
	`"severity":"high","detail":"d","steps":[],"labels":[]}],"impacted_areas":[],"evidence":[],"mitigations":[],` +
	`"open_questions":[]}`

func TestDeliverAPI(t *testing.T) {
	ts, s := newTestServer(t, apiAnalysis)
	s.svc.Secrets = secrets.Map{"GH": "gh-SECRET", "BAD": "bad-SECRET"}
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"number":1,"html_url":"https://gh.test/1"}`)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `bad credentials bad-SECRET`)
	}))
	defer bad.Close()
	for _, in := range []config.Integration{
		{Name: "gh", Kind: config.IntegrationGitHub, BaseURL: good.URL, Project: "a/b", TokenRef: "GH"},
		{Name: "broken", Kind: config.IntegrationGitHub, BaseURL: bad.URL, Project: "a/b", TokenRef: "BAD"},
		{Name: "share", Kind: config.IntegrationFolder, Path: t.TempDir()},
	} {
		if _, err := s.svc.AddIntegration(in); err != nil {
			t.Fatal(err)
		}
	}
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Pay","workType":"analyze"}`)
	nb, _ := s.svc.OpenNotebook("pay")
	out, err := s.svc.Run(context.Background(), service.RunRequest{Notebook: nb, Save: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	base := "/v1/notebooks/pay/outputs/" + out.OutputID
	if code, body := call(t, ts, "POST", base+"/issues/push", `{"integration":"gh"}`); code != 200 ||
		!strings.Contains(body, "https://gh.test/1") {
		t.Fatalf("push: %d %s", code, body)
	}
	code, body := call(t, ts, "POST", base+"/issues/push", `{"integration":"broken"}`)
	if code != 502 || strings.Contains(body, "bad-SECRET") {
		t.Fatalf("a tracker failure is a 502 without the token: %d %s", code, body)
	}
	if code, body := call(t, ts, "POST", base+"/publish", `{"integration":"share"}`); code != 200 ||
		!strings.Contains(body, `"finding":-1`) {
		t.Fatalf("publish: %d %s", code, body)
	}
	code, body = call(t, ts, "GET", base+"/deliveries", "")
	if code != 200 || strings.Count(body, `"integration"`) != 2 {
		t.Fatalf("deliveries: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/integrations/share/test", ""); code != 200 {
		t.Fatalf("test: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/integrations/broken/test", ""); code != 502 {
		t.Fatalf("test of a broken one: %d", code)
	}
}
