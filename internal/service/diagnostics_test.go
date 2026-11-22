package service

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsBundleHoldsNoSecretsOrContent(t *testing.T) {
	s := newSvc(t, summary)
	store := memStore{"TESSIVO_LITELLM_KEY": "sk-supersecretvalue123456"}
	s.SecretStore = store
	if _, err := s.CreateNotebook("Confidential merger plan", "summarize"); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "core.log")
	_ = os.WriteFile(logPath, []byte("started\nrequest failed: Bearer sk-leakedtoken0987654321\n"), 0o600)
	var buf bytes.Buffer
	if err := s.Diagnostics(&buf, "v1.2.3", logPath); err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range r.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		files[f.Name] = string(b)
	}
	for _, name := range []string{"about.txt", "checks.txt", "config.json", "core.log"} {
		if _, ok := files[name]; !ok {
			t.Fatalf("missing %s (have %v)", name, files)
		}
	}
	parts := []string{files["about.txt"], files["checks.txt"], files["config.json"], files["core.log"]}
	all := strings.Join(parts, "\n")
	for _, leak := range []string{"sk-supersecretvalue123456", "sk-leakedtoken0987654321", "Confidential merger plan"} {
		if strings.Contains(all, leak) {
			t.Fatalf("diagnostics leak %q", leak)
		}
	}
	for _, want := range []string{"v1.2.3", "TESSIVO_LITELLM_KEY: set", "notebooks: 1", "started"} {
		if !strings.Contains(all, want) {
			t.Errorf("diagnostics lack %q", want)
		}
	}
}
