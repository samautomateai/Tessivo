package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/service"
)

// sseEvents splits an SSE body into (event, data) pairs.
func sseEvents(body string) [][2]string {
	var out [][2]string
	for _, block := range strings.Split(body, "\n\n") {
		var ev, data string
		for _, line := range strings.Split(block, "\n") {
			if v, ok := strings.CutPrefix(line, "event: "); ok {
				ev = v
			} else if v, ok := strings.CutPrefix(line, "data: "); ok {
				data = v
			}
		}
		if ev != "" {
			out = append(out, [2]string{ev, data})
		}
	}
	return out
}

func TestChatAPI(t *testing.T) {
	ts, _ := newTestServer(t, "Hello there")
	code, body := call(t, ts, "POST", "/v1/chats", `{"title":"Ops","thinkingEffort":"low"}`)
	if code != 201 {
		t.Fatalf("create: %d %s", code, body)
	}
	var sess chat.Session
	_ = json.Unmarshal([]byte(body), &sess)

	code, body = call(t, ts, "POST", "/v1/chats/"+sess.ID+"/messages", `{"text":"hi"}`)
	ev := sseEvents(body)
	if code != 200 || len(ev) != 2 || ev[0][0] != "delta" || ev[1][0] != "finished" {
		t.Fatalf("send: %d %q", code, body)
	}
	var reply service.ChatReply
	if err := json.Unmarshal([]byte(ev[1][1]), &reply); err != nil || reply.Assistant.Content != "Hello there" {
		t.Fatalf("finished: %v %+v", err, reply)
	}

	if code, body = call(t, ts, "PATCH", "/v1/chats/"+sess.ID, `{"pinned":true}`); code != 200 ||
		!strings.Contains(body, `"pinned":true`) {
		t.Fatalf("patch: %d %s", code, body)
	}
	code, body = call(t, ts, "GET", "/v1/chats?q=ops", "")
	if code != 200 || !strings.Contains(body, `"messageCount":2`) {
		t.Fatalf("list: %d %s", code, body)
	}
	code, body = call(t, ts, "GET", "/v1/chats/"+sess.ID, "")
	if code != 200 || !strings.Contains(body, "Hello there") {
		t.Fatalf("get: %d %s", code, body)
	}
	if code, body = call(t, ts, "PUT", "/v1/chat-memory", `{"memory":["I run ops"]}`); code != 200 ||
		!strings.Contains(body, "I run ops") {
		t.Fatalf("set memory: %d %s", code, body)
	}
	if code, body = call(t, ts, "GET", "/v1/chat-memory", ""); code != 200 || !strings.Contains(body, "I run ops") {
		t.Fatalf("memory: %d %s", code, body)
	}
	if code, _ = call(t, ts, "DELETE", "/v1/chats/"+sess.ID, ""); code != 204 {
		t.Fatalf("delete: %d", code)
	}
}

func TestChatAPIErrors(t *testing.T) {
	ts, _ := newTestServer(t, "x")
	_, body := call(t, ts, "POST", "/v1/chats", `{}`)
	var sess chat.Session
	_ = json.Unmarshal([]byte(body), &sess)
	missing := "/v1/chats/ffffffffffffffff"
	cases := []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/v1/chats", `{"model":"nope"}`, 400},
		{"POST", "/v1/chats", `{"bogus":1}`, 400},
		{"GET", missing, "", 404},
		{"PATCH", missing, `{"pinned":true}`, 404},
		{"PATCH", "/v1/chats/" + sess.ID, `{"thinkingEffort":"max"}`, 400},
		{"PATCH", "/v1/chats/" + sess.ID, `[`, 400},
		{"DELETE", missing, "", 404},
		{"POST", missing + "/messages", `{"text":"hi"}`, 404},
		{"POST", "/v1/chats/" + sess.ID + "/messages", `{"text":""}`, 400},
		{"POST", "/v1/chats/" + sess.ID + "/messages", `{`, 400},
		{"PUT", "/v1/chat-memory", `{"memory":["` + strings.Repeat("m", chat.MaxMemoryItem+1) + `"]}`, 400},
		{"PUT", "/v1/chat-memory", `nope`, 400},
		{"GET", "/v1/chats?q=" + strings.Repeat("q", chat.MaxQuery+1), "", 400},
	}
	for _, c := range cases {
		if code, body := call(t, ts, c.method, c.path, c.body); code != c.want {
			t.Errorf("%s %s: got %d want %d (%s)", c.method, c.path, code, c.want, body)
		}
	}
}

func TestChatAPIStreamsModelErrors(t *testing.T) {
	ts, s := newTestServer(t, "x")
	s.svc.Config.Defaults.Model = "litellm/m"
	_, body := call(t, ts, "POST", "/v1/chats", `{}`)
	var sess chat.Session
	_ = json.Unmarshal([]byte(body), &sess)
	s.svc.NewModel = nil // the real factory needs an API key that is not set
	s.svc.Secrets = failingSecrets{}
	_, body = call(t, ts, "POST", "/v1/chats/"+sess.ID+"/messages", `{"text":"hi"}`)
	ev := sseEvents(body)
	if len(ev) != 1 || ev[0][0] != "error" || !strings.Contains(ev[0][1], "model setup failed") {
		t.Fatalf("events %q", body)
	}
}

func TestClassifyConflict(t *testing.T) {
	if code, _ := classify(service.ErrConflict); code != 409 {
		t.Fatalf("conflict → %d", code)
	}
}

type failingSecrets struct{}

func (failingSecrets) Get(string) (string, error) { return "", errSecret }

var errSecret = errorString("secret not set")

type errorString string

func (e errorString) Error() string { return string(e) }
