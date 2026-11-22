package mcp

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReadLineDelimited(t *testing.T) {
	r := newReader(strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n" +
		"{\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\"}\n"))
	m, err := r.read()
	if err != nil || string(m.ID) != "1" || string(m.Result) != "{}" {
		t.Fatalf("first: %+v %v", m, err)
	}
	m, err = r.read()
	if err != nil || m.Method != "notifications/tools/list_changed" || m.ID != nil {
		t.Fatalf("second: %+v %v", m, err)
	}
	if _, err := r.read(); !errors.Is(err, io.EOF) {
		t.Fatalf("end: %v", err)
	}
}

func TestReadHeaderFramed(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":"abc","method":"ping"}`
	r := newReader(strings.NewReader("Content-Length: " + itoa(len(body)) + "\r\n\r\n" + body))
	m, err := r.read()
	if err != nil || m.Method != "ping" || string(m.ID) != `"abc"` {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestReadTooLarge(t *testing.T) {
	r := newReader(io.LimitReader(repeatReader('x'), 5<<20))
	if _, err := r.read(); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
	r = newReader(strings.NewReader("Content-Length: 99999999\r\n\r\n"))
	if _, err := r.read(); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("declared length: %v", err)
	}
}

func TestReadBadJSON(t *testing.T) {
	_, err := newReader(strings.NewReader("{not json}\n")).read()
	if err == nil || !strings.Contains(err.Error(), "JSON-RPC") {
		t.Fatalf("bad json: %v", err)
	}
}

func TestWriteRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	id := int64(7)
	if err := writeMessage(&buf, request(id, "tools/list", map[string]string{"cursor": "c"})); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(buf.String(), "\n") || strings.Count(buf.String(), "\n") != 1 {
		t.Fatalf("framing: %q", buf.String())
	}
	m, err := newReader(&buf).read()
	if err != nil || m.Method != "tools/list" || string(m.ID) != "7" || m.JSONRPC != "2.0" {
		t.Fatalf("%+v %v", m, err)
	}
}

func FuzzReadMessage(f *testing.F) {
	f.Add([]byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n"))
	f.Add([]byte("Content-Length: 2\r\n\r\n{}"))
	f.Add([]byte("Content-Length: -1\r\n\r\n"))
	f.Add([]byte("[1,2]\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := newReader(bytes.NewReader(data)).read()
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if err := writeMessage(&buf, m); err != nil {
			t.Fatalf("accepted message does not re-encode: %v", err)
		}
	})
}

type repeatReader byte

func (b repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

func itoa(n int) string {
	var buf [20]byte
	i := len(buf)
	for {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
		if n == 0 {
			return string(buf[i:])
		}
	}
}
