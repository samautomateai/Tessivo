package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/secrets"
	"github.com/samautomateai/tessivo/internal/service"
)

type memStore map[string]string

func (m memStore) Get(ref string) (string, error) {
	if v, ok := m[ref]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (m memStore) Set(ref, v string) error { m[ref] = v; return nil }
func (m memStore) Delete(ref string) error { delete(m, ref); return nil }

func TestSettingsAndSecretsAPI(t *testing.T) {
	ts, s := newTestServer(t, summary)
	store := memStore{}
	s.svc.SecretStore = store
	cur := s.svc.Settings()
	cur.Defaults.Model = "litellm/qwen"
	body, _ := json.Marshal(service.SettingsUpdate{
		Providers: cur.Providers, Aliases: cur.Aliases, Defaults: cur.Defaults, Limits: cur.Limits,
	})
	if code, out := call(t, ts, "PUT", "/v1/settings", string(body)); code != 200 ||
		!strings.Contains(out, `"model":"litellm/qwen"`) {
		t.Fatalf("put settings: %d %s", code, out)
	}
	if code, _ := call(t, ts, "PUT", "/v1/settings", `{"defaults":{"thinking_effort":"x"}}`); code != 400 {
		t.Fatalf("invalid settings: %d", code)
	}
	if code, out := call(t, ts, "GET", "/v1/secrets/MY_KEY", ""); code != 200 || !strings.Contains(out, `"set":false`) {
		t.Fatalf("status: %d %s", code, out)
	}
	if code, _ := call(t, ts, "PUT", "/v1/secrets/MY_KEY", `{"value":"sk-abc"}`); code != 204 {
		t.Fatalf("set: %d", code)
	}
	code, out := call(t, ts, "GET", "/v1/secrets/MY_KEY", "")
	if code != 200 || !strings.Contains(out, `"set":true`) || strings.Contains(out, "sk-abc") {
		t.Fatalf("the key must never come back: %d %s", code, out)
	}
	if code, _ := call(t, ts, "PUT", "/v1/secrets/bad%20name", `{"value":"x"}`); code != 400 {
		t.Fatalf("bad name: %d", code)
	}
	if code, _ := call(t, ts, "DELETE", "/v1/secrets/MY_KEY", ""); code != 204 || store["MY_KEY"] != "" {
		t.Fatalf("delete: %d", code)
	}
	s.svc.SecretStore = nil
	if code, _ := call(t, ts, "PUT", "/v1/secrets/MY_KEY", `{"value":"x"}`); code != 503 {
		t.Fatalf("no keychain: %d", code)
	}
}

func TestModelTestRoute(t *testing.T) {
	ts, _ := newTestServer(t, "OK")
	code, body := call(t, ts, "POST", "/v1/models/test", `{"model":"litellm/m"}`)
	if code != 200 || !strings.Contains(body, `"model":"litellm/m"`) || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("test: %d %s", code, body)
	}
	if code, _ := call(t, ts, "POST", "/v1/models/test", `{"model":"nope"}`); code != 400 {
		t.Fatalf("unknown model: %d", code)
	}
}
