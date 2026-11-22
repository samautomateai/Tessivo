package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResearchAPIFlow(t *testing.T) {
	ts, _ := newTestServer(t, "Ship in March [1].")
	code, body := call(t, ts, "POST", "/v1/research", `{"name":"Board","description":"Q3"}`)
	if code != 201 || !strings.Contains(body, `"id":"board"`) {
		t.Fatalf("create: %d %s", code, body)
	}
	src := filepath.Join(t.TempDir(), "minutes.md")
	_ = os.WriteFile(src, []byte("Decided to ship in March."), 0o600)
	b, _ := json.Marshal(map[string]string{"path": src})
	if code, body = call(t, ts, "POST", "/v1/research/board/sources", string(b)); code != 200 {
		t.Fatalf("add source: %d %s", code, body)
	}
	var nb struct {
		Sources []struct {
			ID, Level, Summary string
		} `json:"sources"`
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, body = call(t, ts, "GET", "/v1/research/board", "")
		_ = json.Unmarshal([]byte(body), &nb)
		if len(nb.Sources) == 1 && nb.Sources[0].Summary == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("summary never ready: %s", body)
		}
		time.Sleep(50 * time.Millisecond)
	}
	sid := nb.Sources[0].ID
	if code, body = call(t, ts, "PATCH", "/v1/research/board/sources/"+sid, `{"level":"full"}`); code != 200 ||
		!strings.Contains(body, `"level":"full"`) {
		t.Fatalf("level: %d %s", code, body)
	}
	if code, body = call(t, ts, "GET", "/v1/research/board/context", ""); code != 200 ||
		!strings.Contains(body, `"sources":1`) {
		t.Fatalf("context: %d %s", code, body)
	}
	code, body = call(t, ts, "POST", "/v1/research/board/chats", `{"title":"Plans"}`)
	var chat struct{ ID string }
	_ = json.Unmarshal([]byte(body), &chat)
	if code != 201 || chat.ID == "" {
		t.Fatalf("chat: %d %s", code, body)
	}
	_, body = call(t, ts, "POST", "/v1/research/board/chats/"+chat.ID+"/messages", `{"text":"When?"}`)
	ev := sseEvents(body)
	last := ev[len(ev)-1]
	if last[0] != "finished" || !strings.Contains(last[1], `"citations":[{"n":1`) {
		t.Fatalf("send: %s", body)
	}
	var fin struct {
		Assistant struct{ ID string } `json:"assistant"`
	}
	_ = json.Unmarshal([]byte(last[1]), &fin)
	code, body = call(t, ts, "POST", "/v1/research/board/chats/"+chat.ID+"/messages/"+fin.Assistant.ID+"/note", "")
	if code != 201 || !strings.Contains(body, `"origin":"ai"`) {
		t.Fatalf("note: %d %s", code, body)
	}
	code, body = call(t, ts, "GET", "/v1/research/board/notes", "")
	if code != 200 || !strings.Contains(body, "minutes.md") {
		t.Fatalf("notes: %d %s", code, body)
	}
	if code, body = call(t, ts, "GET", "/v1/research/board/chats/"+chat.ID, ""); code != 200 ||
		!strings.Contains(body, `"citations"`) {
		t.Fatalf("get chat: %d %s", code, body)
	}
}

func TestResearchAPIErrors(t *testing.T) {
	ts, _ := newTestServer(t, "x")
	if code, _ := call(t, ts, "GET", "/v1/research/nope", ""); code != 404 {
		t.Fatalf("unknown: %d", code)
	}
	call(t, ts, "POST", "/v1/research", `{"name":"Ops"}`)
	if code, _ := call(t, ts, "POST", "/v1/research/ops/sources", `{"path":"relative.md"}`); code != 400 {
		t.Fatalf("relative path: %d", code)
	}
	const unknown = "/v1/research/ops/sources/20261122T100000-abcdef"
	if code, _ := call(t, ts, "PATCH", unknown, `{"level":"off"}`); code != 404 {
		t.Fatalf("unknown source: %d", code)
	}
	if code, _ := call(t, ts, "POST", "/v1/research", `{"name":""}`); code != 400 {
		t.Fatalf("empty name: %d", code)
	}
}
