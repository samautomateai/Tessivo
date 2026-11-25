package pack

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadManifest(f *testing.F) {
	f.Add([]byte(`{"formatVersion":1,"id":"team","name":"Team","required":["template:a"]}`))
	f.Add([]byte(`{"formatVersion":1,"id":"x","name":"y","recommended":["mcp:"]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ManifestFile), b, 0o600); err != nil {
			t.Fatal(err)
		}
		p, err := Load(dir) // must not panic
		if err == nil && !idPattern.MatchString(p.Manifest.ID) {
			t.Fatalf("loaded a pack with id %q", p.Manifest.ID)
		}
	})
}
