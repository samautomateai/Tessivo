package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// Suggestion tools offered to a thread's model.
const (
	toolSuggestMemory = "suggest_memory"
	toolSuggestPrompt = "suggest_prompt"
)

// maxSuggestionsPerReply bounds what one reply may suggest; extra calls are dropped.
const maxSuggestionsPerReply = 3

const suggestionParams = `{"type":"object","required":["text"],"additionalProperties":false,"properties":{` +
	`"text":{"type":"string"},"reason":{"type":"string","description":"one sentence on why"}}}`

var suggestionTools = []llm.ToolDef{
	{
		Name: toolSuggestMemory,
		Description: "Suggest a short fact to pin to the notebook's memory, so every future run respects it. " +
			"Use only for a lasting preference or correction. The user decides whether to apply it.",
		Parameters: json.RawMessage(suggestionParams),
	},
	{
		Name: toolSuggestPrompt,
		Description: "Suggest a complete replacement for the notebook's prompt, keeping its {{variables}}. " +
			"Use only when the prompt itself should change. The user decides whether to apply it.",
		Parameters: json.RawMessage(suggestionParams),
	},
}

// parseSuggestionCall turns a tool call into an unsaved suggestion. Calls are
// model output: unknown tools, bad JSON and oversized or empty text are errors.
func parseSuggestionCall(c llm.ToolCall) (notebook.Suggestion, error) {
	var kind string
	switch c.Name {
	case toolSuggestMemory:
		kind = notebook.SuggestMemory
	case toolSuggestPrompt:
		kind = notebook.SuggestPrompt
	default:
		return notebook.Suggestion{}, fmt.Errorf("unknown tool %q", truncateRunes(c.Name, 64))
	}
	if len(c.Arguments) > llm.MaxToolCallArgs {
		return notebook.Suggestion{}, errors.New("arguments are too large")
	}
	var args struct {
		Text   string `json:"text"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(c.Arguments), &args); err != nil {
		return notebook.Suggestion{}, errors.New("arguments are not a JSON object with text")
	}
	sg := notebook.Suggestion{Kind: kind, Text: strings.TrimSpace(args.Text), Reason: strings.TrimSpace(args.Reason)}
	return sg, sg.Check(MaxMemoryItem)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// promptDigest identifies a prompt's text, so a stale suggestion is detected.
func promptDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

const toolRecorded = "recorded"

// acceptCalls parses the calls, keeps the first maxSuggestionsPerReply valid
// ones (redacted, with the target prompt's digest for prompt suggestions) and
// returns one tool result per call.
func (s *Service) acceptCalls(ref outputRef, messageID string, calls []llm.ToolCall,
) ([]notebook.Suggestion, []llm.Message) {
	redact := s.redactor()
	base := sync.OnceValue(func() string {
		if tg, err := s.currentTarget(ref); err == nil {
			return promptDigest(tg.Current)
		}
		return ""
	})
	var accepted []notebook.Suggestion
	results := make([]llm.Message, 0, len(calls))
	for _, c := range calls {
		sg, err := parseSuggestionCall(c)
		result := toolRecorded
		switch {
		case err != nil:
			result = "not recorded: " + err.Error()
		case len(accepted) >= maxSuggestionsPerReply:
			result = fmt.Sprintf("not recorded: at most %d suggestions per reply", maxSuggestionsPerReply)
		default:
			sg.ID, sg.MessageID, sg.CreatedAt = notebook.NewID(), messageID, nowRFC3339()
			sg.Status, sg.Text, sg.Reason = notebook.SuggestOpen, redact(sg.Text), redact(sg.Reason)
			if sg.Kind == notebook.SuggestPrompt {
				sg.Base = base()
			}
			accepted = append(accepted, sg)
		}
		results = append(results, llm.Message{Role: llm.RoleTool, ToolCallID: c.ID, Content: result})
	}
	return accepted, results
}

// recordSuggestions stores up to maxSuggestionsPerReply valid suggestions
// (redacted) and returns them with one tool result per call. A store failure
// (ErrConflict when the open cap is reached) is returned; nothing is recorded.
func (s *Service) recordSuggestions(ref outputRef, messageID string, calls []llm.ToolCall,
	onSuggestion func(notebook.Suggestion),
) ([]notebook.Suggestion, []llm.Message, error) {
	accepted, results := s.acceptCalls(ref, messageID, calls)
	if len(accepted) == 0 {
		return nil, results, nil
	}
	if err := ref.nb.UpdateSuggestions(ref.out.ID, func(list []notebook.Suggestion) ([]notebook.Suggestion, error) {
		return append(list, accepted...), nil
	}); err != nil {
		for i := range results {
			if results[i].Content == toolRecorded {
				results[i].Content = "not recorded: " + err.Error()
			}
		}
		if errors.Is(err, notebook.ErrTooManySuggestions) {
			err = fmt.Errorf("%w: %w; dismiss some first", ErrConflict, err)
		}
		return nil, results, err
	}
	for _, sg := range accepted {
		if onSuggestion != nil {
			onSuggestion(sg)
		}
	}
	return accepted, results, nil
}
