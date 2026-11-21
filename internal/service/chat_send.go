package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/samautomateai/tessivo/internal/audit"
	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
)

// KindChatMessage is the audit entry for one chat reply.
const KindChatMessage = "chat.message"

// DefaultChatBudget is the context budget (tokens) when none is configured.
const DefaultChatBudget = 64000

// ChatSend is one user message.
type ChatSend struct {
	Text        string            `json:"text"`
	Attachments []chat.Attachment `json:"attachments"`
}

// ChatReply is the result of SendChat: the stored user and assistant turns.
type ChatReply struct {
	User      chat.Message `json:"user"`
	Assistant chat.Message `json:"assistant"`
}

// CheckChatSend validates a message for a session without sending it, so
// transports can reject bad requests before they start streaming.
func (s *Service) CheckChatSend(id string, in ChatSend) error {
	if err := chat.CheckMessage(in.Text, in.Attachments); err != nil {
		return chatErr(err)
	}
	_, err := s.GetChat(id)
	return err
}

// SendChat stores the user's message, streams the reply through onDelta and
// stores it. Cancelling ctx stops the reply; the partial text is kept and
// marked stopped. A model failure is stored on the reply and returned.
func (s *Service) SendChat(ctx context.Context, id string, in ChatSend, onDelta func(string)) (ChatReply, error) {
	if err := chat.CheckMessage(in.Text, in.Attachments); err != nil {
		return ChatReply{}, chatErr(err)
	}
	st, err := s.chatStore()
	if err != nil {
		return ChatReply{}, err
	}
	release, err := st.Acquire(id)
	if err != nil {
		return ChatReply{}, chatErr(err)
	}
	defer release()
	sess, err := st.Get(id)
	if err != nil {
		return ChatReply{}, chatErr(err)
	}
	settings := s.chatModelSettings(sess)
	if settings.Model == "" {
		return ChatReply{}, invalidf("no model set; choose one for this chat or set defaults.model in the config")
	}
	alias, err := s.Config.ResolveModel(settings.Model)
	if err != nil {
		return ChatReply{}, &InvalidError{err.Error()}
	}
	atts, docs, err := s.resolveAttachments(ctx, in.Attachments)
	if err != nil {
		return ChatReply{}, err
	}
	memory, err := st.Memory()
	if err != nil {
		return ChatReply{}, chatErr(err)
	}
	user := chat.Message{
		ID: chat.NewMessageID(), Role: chat.RoleUser, Content: in.Text, CreatedAt: nowRFC3339(),
		Attachments: atts, Tokens: extract.EstimateTokens(in.Text),
	}
	sess, err = st.Update(id, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, user)
		sess.AutoTitle()
		return nil
	})
	if err != nil {
		return ChatReply{}, chatErr(err)
	}
	docs = append(docs, s.sessionDocs(ctx, sess, user.ID)...)
	req := llm.Request{
		Model: settings.Model, MaxTokens: settings.MaxOutputTokens, Temperature: settings.Temperature,
		ReasoningEffort: settings.ThinkingEffort,
		Messages:        chat.Build(sess, memory, docs, budget(settings)),
	}
	reply, runErr := s.streamReply(ctx, alias, req, onDelta)
	if _, err := st.Update(id, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, reply)
		return nil
	}); err != nil {
		return ChatReply{User: user, Assistant: reply}, errors.Join(runErr, chatErr(err))
	}
	auditErr := s.auditChat(id, user, reply, settings)
	return ChatReply{User: user, Assistant: reply}, errors.Join(runErr, auditErr)
}

func budget(r config.RunSettings) int {
	if r.ContextBudget != nil && *r.ContextBudget > 0 {
		return *r.ContextBudget
	}
	return DefaultChatBudget
}

// streamReply runs the model and turns the outcome into the stored reply.
func (s *Service) streamReply(ctx context.Context, alias config.Alias, req llm.Request, onDelta func(string),
) (chat.Message, error) {
	reply := chat.Message{ID: chat.NewMessageID(), Role: chat.RoleAssistant, CreatedAt: nowRFC3339()}
	model, err := s.aliasModel(alias)
	if err != nil {
		reply.Stopped, reply.Error = true, "model setup failed"
		return reply, err
	}
	res, text, err := streamText(ctx, model, req, 0, onDelta)
	reply.Model, reply.Content, reply.Tokens = res.Target, text, res.Usage.CompletionTokens
	return reply, finishReply(&reply, err)
}

func (s *Service) aliasModel(alias config.Alias) (executor.Model, error) {
	factory := s.NewModel
	if factory == nil {
		factory = DefaultModelFactory
	}
	return factory(s.Config, alias, s.Secrets)
}

// streamText streams one model call, forwarding deltas until the reply
// (including the used bytes already sent) reaches chat.MaxReplyBytes, and
// returns the reply text.
func streamText(ctx context.Context, model executor.Model, req llm.Request, used int, onDelta func(string),
) (llm.Result, string, error) {
	var sb strings.Builder
	res, err := model.ChatStream(ctx, req, func(d string) {
		if used+sb.Len()+len(d) > chat.MaxReplyBytes {
			return
		}
		sb.WriteString(d)
		if onDelta != nil {
			onDelta(d)
		}
	})
	text := res.Content
	if text == "" || used+len(text) > chat.MaxReplyBytes {
		text = sb.String()
	}
	return res, text, err
}

// finishReply marks a reply stopped or failed after err; a stop by the
// caller is not an error.
func finishReply(reply *chat.Message, err error) error {
	if err == nil {
		return nil
	}
	reply.Stopped = true
	if stopped(err) {
		return nil
	}
	reply.Error = classifyChatError(err)
	return fmt.Errorf("chat reply: %w", err)
}

// classifyChatError stores a short, secret-free reason on a failed reply.
func classifyChatError(err error) string {
	if c := llm.Classify(err); c != "" {
		return "model error: " + c
	}
	return "model error"
}

func (s *Service) auditChat(id string, user, reply chat.Message, r config.RunSettings) error {
	return s.auditMessage(KindChatMessage, id, user, reply, r)
}

// auditMessage records one reply by digest and size only.
func (s *Service) auditMessage(kind, id string, user, reply chat.Message, r config.RunSettings) error {
	if s.UserAuditPath == "" {
		return nil
	}
	actor := s.Actor
	if actor == "" {
		actor = defaultActor
	}
	l, err := audit.Open(s.UserAuditPath, actor)
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	sources := make([]string, 0, len(user.Attachments))
	for _, a := range user.Attachments {
		sources = append(sources, a.Kind+":"+a.Ref)
	}
	err = l.Append(kind, id, map[string]any{
		"prompt_sha256": audit.Digest(user.Content), "prompt_chars": len(user.Content), "sources": sources,
		"model": r.Model, "thinking": r.ThinkingEffort, "answered_by": reply.Model,
		"reply_chars": len(reply.Content), "stopped": reply.Stopped, "error": reply.Error,
	})
	return errors.Join(err, l.Close())
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// resolveAttachments loads each attachment's text and records its size.
func (s *Service) resolveAttachments(ctx context.Context, in []chat.Attachment,
) ([]chat.Attachment, []extract.Document, error) {
	out := make([]chat.Attachment, 0, len(in))
	var docs []extract.Document
	for _, a := range in {
		d, label, err := s.attachmentDocs(ctx, a)
		if err != nil {
			return nil, nil, err
		}
		a.Tokens = 0
		for _, doc := range d {
			a.Tokens += doc.Tokens()
		}
		if a.Label == "" {
			a.Label = label
		}
		out = append(out, a)
		docs = append(docs, d...)
	}
	return out, docs, nil
}

func (s *Service) attachmentDocs(ctx context.Context, a chat.Attachment) ([]extract.Document, string, error) {
	switch a.Kind {
	case chat.AttachPath:
		if !filepath.IsAbs(a.Ref) {
			return nil, "", invalidf("attachment path must be absolute")
		}
		d, err := extract.Path(a.Ref)
		if err != nil {
			return nil, "", &InvalidError{fmt.Sprintf("attachment %s: %v", filepath.Base(a.Ref), err)}
		}
		return d, filepath.Base(a.Ref), nil
	case chat.AttachNotebook:
		nb, err := s.OpenNotebook(a.Ref)
		if err != nil {
			return nil, "", err
		}
		d, err := s.notebookDocs(ctx, nb)
		return d, nb.Manifest.Name, err
	default: // chat.AttachOutput; CheckMessage rejects other kinds
		nbID, outID, ok := strings.Cut(a.Ref, "/")
		if !ok {
			return nil, "", invalidf("output attachments use notebookID/outputID")
		}
		o, err := s.GetOutput(nbID, outID)
		if err != nil {
			return nil, "", err
		}
		return []extract.Document{{Source: o.Title, Kind: docMarkdown, Text: o.Markdown}}, o.Title, nil
	}
}

// sessionDocs re-reads context attached to earlier messages so later turns can
// still see it. Attachments that no longer resolve are skipped, not fatal.
func (s *Service) sessionDocs(ctx context.Context, sess chat.Session, skipID string) []extract.Document {
	seen := map[string]bool{}
	var docs []extract.Document
	for _, m := range sess.Messages {
		if m.ID == skipID {
			continue
		}
		for _, a := range m.Attachments {
			key := a.Kind + "\x00" + a.Ref
			if seen[key] {
				continue
			}
			seen[key] = true
			if d, _, err := s.attachmentDocs(ctx, a); err == nil {
				docs = append(docs, d...)
			}
		}
	}
	return docs
}
