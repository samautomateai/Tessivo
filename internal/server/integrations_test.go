package server

import (
	"strings"
	"testing"
)

func TestIntegrationsAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	body := `{"name":"gh","kind":"github","project":"acme/shop","token_ref":"GH_TOKEN"}`
	code, got := call(t, ts, "POST", "/v1/integrations", body)
	if code != 201 || !strings.Contains(got, `"tokenSet":false`) {
		t.Fatalf("add: %d %s", code, got)
	}
	if code, _ := call(t, ts, "POST", "/v1/integrations", body); code != 409 {
		t.Fatalf("duplicate: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/integrations", `{"name":"x","kind":"trello"}`); code != 400 {
		t.Fatalf("invalid: %d", code)
	}
	if code, got := call(t, ts, "GET", "/v1/integrations", ""); code != 200 || !strings.Contains(got, `"name":"gh"`) {
		t.Fatalf("list: %d %s", code, got)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/integrations/gh", ""); code != 204 {
		t.Fatalf("remove: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/integrations/gh", ""); code != 404 {
		t.Fatalf("remove twice: %d", code)
	}
}
