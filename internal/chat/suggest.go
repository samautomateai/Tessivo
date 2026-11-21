package chat

import (
	"strings"
	"unicode"
)

// workTypeCues are words that point at a work type. Matching is on whole
// lowercase words or word prefixes ("summar" matches "summarize").
var workTypeCues = map[string][]string{
	"analyze":   {"analy", "risk", "assess", "impact", "evaluat", "compar", "trend", "why", "root", "audit"},
	"summarize": {"summar", "tldr", "digest", "overview", "recap", "brief", "key", "takeaway", "condense"},
	"document":  {"document", "explain", "how", "architecture", "onboard", "guide", "readme", "describe", "spec"},
	"report":    {"report", "weekly", "monthly", "status", "progress", "kpi", "metric", "period", "quarter"},
}

// workTypeOrder breaks ties deterministically.
var workTypeOrder = []string{"summarize", "analyze", "document", "report"}

// SuggestWorkType picks the work type whose cues appear most often in the
// session's user messages, preferring Summarize when nothing stands out.
func SuggestWorkType(s Session) string {
	counts := map[string]int{}
	for _, w := range Words(UserText(s)) {
		for wt, cues := range workTypeCues {
			for _, c := range cues {
				if strings.HasPrefix(w, c) {
					counts[wt]++
					break
				}
			}
		}
	}
	best := workTypeOrder[0]
	for _, wt := range workTypeOrder[1:] {
		if counts[wt] > counts[best] {
			best = wt
		}
	}
	return best
}

// UserText joins the session's user messages.
func UserText(s Session) string {
	var sb strings.Builder
	for _, m := range s.Messages {
		if m.Role == RoleUser {
			sb.WriteString(m.Content)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// LastUserMessage returns the most recent user message, or "".
func LastUserMessage(s Session) string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == RoleUser {
			return s.Messages[i].Content
		}
	}
	return ""
}

// Find returns the message with id.
func (s Session) Find(id string) (Message, bool) {
	for _, m := range s.Messages {
		if m.ID == id {
			return m, true
		}
	}
	return Message{}, false
}

// QuestionBefore returns the user message that a reply answers.
func (s Session) QuestionBefore(id string) string {
	q := ""
	for _, m := range s.Messages {
		if m.ID == id {
			return q
		}
		if m.Role == RoleUser {
			q = m.Content
		}
	}
	return ""
}

// Attachments returns every distinct attachment in the session, oldest first.
func (s Session) Attachments() []Attachment {
	seen := map[string]bool{}
	var out []Attachment
	for _, m := range s.Messages {
		for _, a := range m.Attachments {
			key := a.Kind + "\x00" + a.Ref
			if !seen[key] {
				seen[key] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// Words splits text into lowercase words of letters and digits, at most
// maxWords of them, so scoring stays cheap on long chats.
func Words(text string) []string {
	const maxWords = 5000
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(words) > maxWords {
		words = words[:maxWords]
	}
	return words
}
