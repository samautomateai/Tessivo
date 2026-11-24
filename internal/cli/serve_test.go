package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/server"
)

func TestServeWritesEndpointAndShutsDown(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	f, _ := os.ReadFile(h.env.ConfigPath)
	var cfg map[string]any
	_ = json.Unmarshal(f, &cfg)
	cfg["workspace_dir"] = filepath.Join(h.dir, "ws")
	b, _ := json.Marshal(cfg)
	_ = os.WriteFile(h.env.ConfigPath, b, 0o600)

	ep := filepath.Join(h.dir, "run", "endpoint.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- Main(ctx, []string{"serve", "--network", "tcp", "--addr", "127.0.0.1:0", "--endpoint-file", ep}, h.env)
	}()
	var e server.Endpoint
	deadline := time.Now().Add(5 * time.Second)
	for {
		if raw, err := os.ReadFile(ep); err == nil && json.Unmarshal(raw, &e) == nil && e.Token != "" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("endpoint file not written")
		}
		time.Sleep(20 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", "http://"+e.Address+"/v1/notebooks", nil)
	req.Header.Set("Authorization", "Bearer "+e.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		cancel()
		t.Fatalf("request: %v %v", err, resp)
	}
	_ = resp.Body.Close()
	req, _ = http.NewRequest("GET", "http://"+e.Address+"/v1/diagnostics", nil)
	req.Header.Set("Authorization", "Bearer "+e.Token)
	if resp, err = http.DefaultClient.Do(req); err != nil || resp.StatusCode != 200 ||
		resp.Header.Get("Content-Type") != "application/zip" {
		cancel()
		t.Fatalf("diagnostics: %v %v", err, resp)
	}
	_ = resp.Body.Close()
	cancel()
	if code := <-done; code != ExitOK {
		t.Fatalf("serve exit %d: %s", code, h.errOut)
	}
	log, err := os.ReadFile(filepath.Join(filepath.Dir(h.env.ConfigPath), "logs", "core.log"))
	if err != nil || !strings.Contains(string(log), "core started") || strings.Contains(string(log), e.Token) {
		t.Fatalf("core log: %v %q", err, log)
	}
	if _, err := os.Stat(ep); !os.IsNotExist(err) {
		t.Fatal("endpoint file should be removed on shutdown")
	}
}

func TestServeErrors(t *testing.T) {
	h := newHarness(t)
	h.initConfig()
	if code := h.run("serve", "--network", "tcp", "--addr", "0.0.0.0:0"); code != ExitError {
		t.Fatal("want non-loopback refusal")
	}
	if code := h.run("serve", "--bogus"); code != ExitUsage {
		t.Fatal("want usage error")
	}
}

func TestWatchParent(t *testing.T) {
	calls := 0
	done := make(chan struct{})
	go watchParent(context.Background(), 10, func() int {
		calls++
		if calls > 2 {
			return 1 // re-parented to init
		}
		return 10
	}, time.Millisecond, func() { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("parent exit not detected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	watchParent(ctx, 10, func() int { return 10 }, time.Hour, func() { t.Fatal("must not fire") })
	pctx, pcancel := exitWithParent(context.Background())
	pcancel()
	<-pctx.Done()
}

// TestServeStartsWithoutConfig: a first run (no config file yet) must not
// stop the desktop app's core; it starts on the defaults, and the first
// settings change writes the file.
func TestServeStartsWithoutConfig(t *testing.T) {
	h := newHarness(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	ep := filepath.Join(h.dir, "run", "endpoint.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- Main(ctx, []string{"serve", "--network", "tcp", "--addr", "127.0.0.1:0", "--endpoint-file", ep}, h.env)
	}()
	var e server.Endpoint
	deadline := time.Now().Add(5 * time.Second)
	for {
		if raw, err := os.ReadFile(ep); err == nil && json.Unmarshal(raw, &e) == nil && e.Token != "" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("serve did not start without a config: %s", h.errOut)
		}
		time.Sleep(20 * time.Millisecond)
	}
	req, _ := http.NewRequest("GET", "http://"+e.Address+"/v1/settings", nil)
	req.Header.Set("Authorization", "Bearer "+e.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		cancel()
		t.Fatalf("settings: %v %v", err, resp)
	}
	_ = resp.Body.Close()
	cancel()
	if code := <-done; code != ExitOK {
		t.Fatalf("serve exit %d: %s", code, h.errOut)
	}
	if _, err := os.Stat(h.env.ConfigPath); !os.IsNotExist(err) {
		t.Fatal("starting must not write a config; the first settings change does")
	}
}
