package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type wireChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content   string         `json:"content"`
			ToolCalls []wireToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
}

// ChatStream sends a streaming request, calling onDelta for each content
// fragment, and returns the assembled response.
func (c *Client) ChatStream(ctx context.Context, req Request, onDelta func(string)) (Response, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := c.do(ctx, http.MethodPost, "/chat/completions", toWire(req, true))
	if err != nil {
		return Response{}, toolsError(req, err)
	}
	if c.Timeout > 0 {
		resp.Body = newIdleReader(resp.Body, c.Timeout, cancel)
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := readSSE(resp, onDelta)
	if err != nil && errors.Is(context.Cause(ctx), ErrIdleTimeout) {
		err = fmt.Errorf("%w after %s: %w", ErrIdleTimeout, c.Timeout, context.DeadlineExceeded)
		return out, &TransportError{Err: err}
	}
	return out, err
}

func readSSE(resp *http.Response, onDelta func(string)) (Response, error) {
	var out Response
	var sb strings.Builder
	acc := &toolAccumulator{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var ch wireChunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			return Response{}, fmt.Errorf("decode stream chunk: %w", err)
		}
		if err := applyChunk(&out, &sb, acc, ch, onDelta); err != nil {
			return Response{}, err
		}
	}
	if err := sc.Err(); err != nil {
		return Response{}, &TransportError{Err: err}
	}
	out.Content = sb.String()
	out.ToolCalls = acc.calls()
	return out, nil
}

func applyChunk(out *Response, sb *strings.Builder, acc *toolAccumulator, ch wireChunk, onDelta func(string)) error {
	if ch.Model != "" {
		out.Model = ch.Model
	}
	if ch.Usage != nil {
		out.Usage = *ch.Usage
	}
	for _, c := range ch.Choices {
		if c.Delta.Content != "" {
			sb.WriteString(c.Delta.Content)
			if onDelta != nil {
				onDelta(c.Delta.Content)
			}
		}
		for _, d := range c.Delta.ToolCalls {
			if err := acc.add(d); err != nil {
				return err
			}
		}
		if c.FinishReason != nil {
			out.FinishReason = *c.FinishReason
		}
	}
	return nil
}
