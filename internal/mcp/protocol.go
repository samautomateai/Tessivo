package mcp

// Ported from Zero internal/mcp/protocol.go (commit 8288ae81bd6a, MIT; see doc.go).

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// MaxMessage bounds one framed message (bytes), whatever its framing says.
const MaxMessage = 4 << 20

const jsonrpcVersion = "2.0"

// ErrTooLarge is returned for a message over MaxMessage.
var ErrTooLarge = errors.New("mcp: message exceeds 4 MiB")

// message is a JSON-RPC 2.0 request, notification or response. ID is kept raw
// so a server's string IDs are echoed back unchanged.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message) }

// request builds a request with a numeric ID (params nil for none).
func request(id int64, method string, params any) message {
	m := message{JSONRPC: jsonrpcVersion, ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method}
	if params != nil {
		m.Params, _ = json.Marshal(params)
	}
	return m
}

type reader struct{ r *bufio.Reader }

func newReader(r io.Reader) *reader { return &reader{r: bufio.NewReader(r)} }

// read returns the next message. Messages are newline-delimited JSON (the MCP
// stdio framing); LSP-style Content-Length framing is also accepted. Blank
// lines between messages are skipped.
func (r *reader) read() (message, error) {
	for {
		line, err := r.readLine()
		trimmed := strings.TrimSpace(line)
		if err != nil {
			if errors.Is(err, io.EOF) && isJSONStart(trimmed) {
				return decode([]byte(trimmed)) // a last message without a newline
			}
			return message{}, err
		}
		switch {
		case trimmed == "":
			continue
		case isJSONStart(trimmed):
			return decode([]byte(trimmed))
		default:
			return r.readHeaderFramed(trimmed)
		}
	}
}

func isJSONStart(s string) bool { return s != "" && (s[0] == '{' || s[0] == '[') }

func decode(body []byte) (message, error) {
	var m message
	if err := json.Unmarshal(body, &m); err != nil {
		return message{}, fmt.Errorf("mcp: invalid JSON-RPC message: %w", err)
	}
	return m, nil
}

// readLine reads one line, refusing to buffer more than MaxMessage bytes.
func (r *reader) readLine() (string, error) {
	var buf []byte
	for {
		chunk, err := r.r.ReadSlice('\n')
		if len(buf)+len(chunk) > MaxMessage {
			return "", ErrTooLarge
		}
		buf = append(buf, chunk...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return string(buf), err
	}
}

// readHeaderFramed reads headers up to a blank line, then Content-Length bytes.
func (r *reader) readHeaderFramed(first string) (message, error) {
	length, line := 0, first
	for {
		name, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "content-length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil || n <= 0 {
				return message{}, fmt.Errorf("mcp: invalid content length %q", strings.TrimSpace(value))
			}
			if n > MaxMessage {
				return message{}, ErrTooLarge
			}
			length = n
		}
		next, err := r.readLine()
		if err != nil {
			return message{}, err
		}
		if line = strings.TrimRight(next, "\r\n"); line == "" {
			break
		}
	}
	if length == 0 {
		return message{}, errors.New("mcp: missing content length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r.r, body); err != nil {
		return message{}, fmt.Errorf("mcp: read message body: %w", err)
	}
	return decode(body)
}

// writeMessage writes m as one line of JSON (JSON never contains a raw newline).
func writeMessage(w io.Writer, m message) error {
	if m.JSONRPC == "" {
		m.JSONRPC = jsonrpcVersion
	}
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("mcp: encode message: %w", err)
	}
	if _, err := w.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("mcp: write message: %w", err)
	}
	return nil
}
