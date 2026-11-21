package audit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/samautomateai/tessivo/internal/testutil"
)

func openTemp(t *testing.T) (*Log, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sub", "audit.jsonl")
	l, err := Open(p, "cli", "sk-live-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, p
}

func TestAppendVerifyAndResume(t *testing.T) {
	l, p := openTemp(t)
	if err := l.Append(KindRunStart, "r1", map[string]any{
		"prompt_sha256": Digest("Summarize {{x}}"), "sources": []string{"a.md", "b.csv"}, "tokens": 12,
		"nested": map[string]any{"ok": true, "ratio": 0.5}, "err": errors.New("auth sk-live-secret-value failed"),
		"other": time.Duration(5), "nothing": nil,
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(KindRunFinish, "r1", nil); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()

	raw, _ := os.ReadFile(p)
	if bytes.Contains(raw, []byte("sk-live-secret-value")) {
		t.Fatal("secret written to audit log")
	}
	testutil.AssertPerm(t, p, 0o600)
	l2, err := Open(p, "desktop")
	if err != nil {
		t.Fatal(err)
	}
	if err := l2.Append(KindOutputSaved, "r1", map[string]any{"output_id": "o1"}); err != nil {
		t.Fatal(err)
	}
	_ = l2.Close()
	n, err := VerifyFile(p)
	if err != nil || n != 3 {
		t.Fatalf("verify: %d %v", n, err)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	l, p := openTemp(t)
	for i := 0; i < 3; i++ {
		if err := l.Append(KindModelAttempt, "r", map[string]any{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	_ = l.Close()
	raw, _ := os.ReadFile(p)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")

	cases := map[string]string{
		"modified":  strings.Replace(string(raw), `"i":1`, `"i":9`, 1),
		"deleted":   lines[0] + "\n" + lines[2] + "\n",
		"reordered": lines[1] + "\n" + lines[0] + "\n",
		"garbage":   string(raw) + "not json\n",
		"relinked":  strings.Replace(string(raw), `"seq":2`, `"seq":7`, 1),
	}
	for name, content := range cases {
		if _, err := Verify(strings.NewReader(content)); !errors.Is(err, ErrTampered) {
			t.Errorf("%s: want ErrTampered, got %v", name, err)
		}
	}
	if _, err := Verify(strings.NewReader(strings.Repeat("x", MaxLineBytes+10))); !errors.Is(err, ErrTampered) {
		t.Fatalf("oversized line: %v", err)
	}
}

func TestConcurrentAppends(t *testing.T) {
	l, p := openTemp(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = l.Append(KindToolCall, fmt.Sprint(i), map[string]any{"tool": "jira.search"})
		}(i)
	}
	wg.Wait()
	_ = l.Close()
	if n, err := VerifyFile(p); err != nil || n != 20 {
		t.Fatalf("verify: %d %v", n, err)
	}
}

func TestLimitsAndErrors(t *testing.T) {
	l, _ := openTemp(t)
	if err := l.Append(KindToolCall, "r", map[string]any{"big": strings.Repeat("a", 10*MaxStringLen)}); err != nil {
		t.Fatal(err)
	}
	many := map[string]any{}
	for i := 0; i < 64; i++ {
		many[fmt.Sprint("k", i)] = strings.Repeat("b", MaxStringLen)
	}
	if err := l.Append(KindToolCall, "r", many); err == nil {
		t.Fatal("want oversized entry error")
	}
	if err := l.Append(KindToolCall, "r", map[string]any{"bad": func() {}}); err != nil {
		t.Fatal("unknown types are stringified, not rejected:", err)
	}
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "c.jsonl")
	_ = os.WriteFile(corrupt, []byte("{oops\n"), 0o600)
	if _, err := Open(corrupt, "cli"); err == nil {
		t.Fatal("want corrupt tail error")
	}
	if _, err := Open(testutil.Unwritable(t, "audit.jsonl"), "cli"); err == nil {
		t.Fatal("want open error")
	}
	if _, err := VerifyFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("want missing file error")
	}
	_ = l.Close()
	if err := l.Close(); err == nil {
		t.Fatal("want double close error")
	}
	if err := l.Append(KindRunStart, "r", nil); err == nil {
		t.Fatal("want write-after-close error")
	}
}

func FuzzVerify(f *testing.F) {
	f.Add([]byte(`{"seq":1,"time":"t","kind":"k","prev":"00","hash":"x"}` + "\n"))
	f.Add([]byte("\n\n{}\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		n, err := Verify(bytes.NewReader(data))
		if err == nil && n > 0 && !bytes.Contains(data, []byte(`"hash"`)) {
			t.Fatalf("verified %d entries without hashes", n)
		}
	})
}

func BenchmarkAppend(b *testing.B) {
	l, err := Open(filepath.Join(b.TempDir(), "a.jsonl"), "bench")
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	data := map[string]any{"target": "litellm/model", "tokens": 1234, "sources": []string{"a", "b"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := l.Append(KindModelAttempt, "r", data); err != nil {
			b.Fatal(err)
		}
	}
}
