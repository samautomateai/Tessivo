package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestScheduleAPI(t *testing.T) {
	ts, _ := newTestServer(t, summary)
	call(t, ts, "POST", "/v1/notebooks", `{"name":"Ops","workType":"report"}`)
	code, body := call(t, ts, "POST", "/v1/notebooks/ops/schedules",
		`{"cron":"0 8 * * MON","prompt":"Weekly report","missed":"run_once"}`)
	if code != 201 || !strings.Contains(body, `"nextRun"`) || !strings.Contains(body, `"enabled":true`) {
		t.Fatalf("add: %d %s", code, body)
	}
	var sch struct{ ID string }
	_ = json.Unmarshal([]byte(body), &sch)
	for _, bad := range []string{
		`{"cron":"nope"}`, `{"cron":"* * * * *","missed":"always"}`,
		`{"cron":"* * * * *","promptName":"missing"}`, `{"cron":"* * * * *","extra":1}`,
	} {
		if code, _ := call(t, ts, "POST", "/v1/notebooks/ops/schedules", bad); code != 400 {
			t.Errorf("%s: want 400, got %d", bad, code)
		}
	}
	path := "/v1/notebooks/ops/schedules/" + sch.ID
	if code, body := call(t, ts, "PATCH", path, `{"enabled":false}`); code != 200 || strings.Contains(body, "nextRun") {
		t.Fatalf("pause: %d %s", code, body)
	}
	if code, _ := call(t, ts, "PATCH", path, `{}`); code != 400 {
		t.Fatal("want 400 without enabled")
	}
	if code, body := call(t, ts, "GET", "/v1/notebooks/ops/schedules", ""); code != 200 ||
		!strings.Contains(body, "0 8 * * MON") {
		t.Fatalf("list: %d %s", code, body)
	}
	if code, _ := call(t, ts, "DELETE", path, ""); code != 204 {
		t.Fatal("delete")
	}
	if code, _ := call(t, ts, "DELETE", path, ""); code != 404 {
		t.Fatal("second delete should 404")
	}
	if code, _ := call(t, ts, "GET", "/v1/notebooks/nope/schedules", ""); code != 404 {
		t.Fatal("unknown notebook")
	}
	if code, _ := call(t, ts, "PATCH", "/v1/notebooks/ops/schedules/zz", `{"enabled":true}`); code != 404 {
		t.Fatal("unknown schedule")
	}
}

func TestEventsStream(t *testing.T) {
	ts, s := newTestServer(t, summary)
	if code, _ := call(t, ts, "GET", "/v1/events", ""); code != 404 {
		t.Fatal("events disabled without a broker")
	}
	s.Events = NewBroker()
	old := keepAlive
	keepAlive = 10 * time.Millisecond
	t.Cleanup(func() { keepAlive = old })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	go func() {
		time.Sleep(30 * time.Millisecond)
		s.Events.Publish(map[string]string{"kind": "schedule.finished", "title": "Weekly"})
	}()
	sc := bufio.NewScanner(resp.Body)
	sawKeepAlive := false
	for sc.Scan() {
		line := sc.Text()
		if line == ": keepalive" {
			sawKeepAlive = true
		}
		if strings.HasPrefix(line, "data: ") && strings.Contains(line, "schedule.finished") {
			break
		}
	}
	if !sawKeepAlive {
		t.Fatal("expected a keepalive comment before the event")
	}
}

func TestBrokerLimits(t *testing.T) {
	b := NewBroker()
	var cancels []func()
	for i := 0; i < MaxSubscribers; i++ {
		_, c, err := b.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
		cancels = append(cancels, c)
	}
	if _, _, err := b.Subscribe(); err == nil {
		t.Fatal("want subscriber limit")
	}
	for i := 0; i < 100; i++ {
		b.Publish(i) // full buffers must not block
	}
	for _, c := range cancels {
		c()
	}
	if _, _, err := b.Subscribe(); err != nil {
		t.Fatal("slots should be free again")
	}
}
