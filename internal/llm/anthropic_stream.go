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

type antEvent struct {
	Type    string      `json:"type"`
	Index   int         `json:"index"`
	Message antResponse `json:"message"`
	Block   antBlock    `json:"content_block"`
	Delta   struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage antUsage `json:"usage"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// antStream assembles a streamed reply: text, tool inputs and usage.
type antStream struct {
	req     Request
	reply   antResponse
	blocks  map[int]*antBlock
	inputs  map[int]*strings.Builder
	order   []int
	onDelta func(string)
}

// ChatStream sends a streaming request; onDelta gets answer text (never thinking).
func (a *Anthropic) ChatStream(ctx context.Context, req Request, onDelta func(string)) (Response, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	resp, err := sendJSON(ctx, a.HTTP, http.MethodPost, a.BaseURL+"/v1/messages", a.headers(), a.toWire(req, true),
		a.APIKey)
	if err != nil {
		return Response{}, toolsError(req, err)
	}
	resp.Body = newIdleReader(resp.Body, a.Timeout, cancel)
	defer func() { _ = resp.Body.Close() }()
	s := &antStream{req: req, blocks: map[int]*antBlock{}, inputs: map[int]*strings.Builder{}, onDelta: onDelta}
	out, err := s.read(resp)
	if err != nil && errors.Is(context.Cause(ctx), ErrIdleTimeout) {
		err = fmt.Errorf("%w after %s: %w", ErrIdleTimeout, a.Timeout, context.DeadlineExceeded)
		return out, &TransportError{Err: err}
	}
	return out, err
}

func (s *antStream) read(resp *http.Response) (Response, error) {
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		data, ok := strings.CutPrefix(sc.Text(), "data:")
		if !ok {
			continue
		}
		var ev antEvent
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &ev); err != nil {
			return Response{}, fmt.Errorf("decode stream event: %w", err)
		}
		done, err := s.apply(ev)
		if err != nil {
			return Response{}, err
		}
		if done {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return Response{}, &TransportError{Err: err}
	}
	for _, i := range s.order {
		b := s.blocks[i]
		if b.Type == blockToolUse {
			b.Input = schemaArgs(s.inputs[i].String())
		}
		s.reply.Content = append(s.reply.Content, *b)
	}
	return antResult(s.req, s.reply)
}

// apply handles one event; it reports true at the end of the message.
func (s *antStream) apply(ev antEvent) (bool, error) {
	switch ev.Type {
	case "message_start":
		s.reply.Model, s.reply.Usage = ev.Message.Model, ev.Message.Usage
	case "content_block_start":
		if len(s.order) >= MaxToolCalls*4 {
			return false, fmt.Errorf("%w: too many content blocks", ErrToolCallLimit)
		}
		b := ev.Block
		s.blocks[ev.Index], s.inputs[ev.Index] = &b, &strings.Builder{}
		s.order = append(s.order, ev.Index)
	case "content_block_delta":
		return false, s.delta(ev)
	case "message_delta":
		s.reply.StopReason = ev.Delta.StopReason
		s.reply.Usage.OutputTokens = ev.Usage.OutputTokens
	case "message_stop":
		return true, nil
	case "error":
		status := 500
		if ev.Error.Type == "overloaded_error" {
			status = 529
		}
		return false, &APIError{Status: status, Body: ev.Error.Type + ": " + ev.Error.Message}
	}
	return false, nil
}

func (s *antStream) delta(ev antEvent) error {
	b, ok := s.blocks[ev.Index]
	if !ok {
		return nil
	}
	switch ev.Delta.Type {
	case "text_delta":
		b.Text += ev.Delta.Text
		if s.onDelta != nil && ev.Delta.Text != "" {
			s.onDelta(ev.Delta.Text)
		}
	case "input_json_delta":
		in := s.inputs[ev.Index]
		if in.Len()+len(ev.Delta.PartialJSON) > MaxToolCallArgs {
			return errTooLargeArgs(b.Name)
		}
		in.WriteString(ev.Delta.PartialJSON)
		if s.req.Format != nil && b.Name == formatTool && s.onDelta != nil && ev.Delta.PartialJSON != "" {
			s.onDelta(ev.Delta.PartialJSON) // structured output streams as JSON text
		}
	}
	return nil
}
