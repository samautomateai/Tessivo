package archive

import (
	"bytes"
	"path/filepath"
	"testing"
)

func FuzzExtract(f *testing.F) {
	var buf bytes.Buffer
	dir := f.TempDir()
	_ = Write(&buf, dir, Meta{Kind: KindNotebook}, func(string) bool { return true })
	f.Add(buf.Bytes())
	f.Add([]byte("PK\x03\x04"))
	f.Fuzz(func(t *testing.T, b []byte) {
		dest := filepath.Join(t.TempDir(), "x")
		_, _ = Extract(bytes.NewReader(b), int64(len(b)), dest) // must not panic or escape
	})
}
