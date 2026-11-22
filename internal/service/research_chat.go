package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/research"
)

// KindResearchMessage is the audit entry for one research answer.
const KindResearchMessage = "research.message"

// ResearchReply is a stored turn with the citations the answer made and the
// sources that could not be read.
type ResearchReply struct {
	ChatReply
	Citations []research.Citation `json:"citations"`
	Skipped   []string            `json:"skipped,omitempty"`
}

func (s *Service) researchChats(id string) (*research.Notebook, *chat.Store, error) {
	nb, err := s.openResearch(id)
	if err != nil {
		return nil, nil, err
	}
	st, err := chat.Open(filepath.Join(nb.Dir, research.ChatsDir))
	if err != nil {
		return nil, nil, fmt.Errorf("research chats: %w", err)
	}
	return nb, st, nil
}

// ListResearchChats lists a notebook's conversations.
func (s *Service) ListResearchChats(id string) ([]chat.Summary, error) {
	_, st, err := s.researchChats(id)
	if err != nil {
		return nil, err
	}
	list, err := st.List(chat.Filter{})
	return list, chatErr(err)
}

// CreateResearchChat starts a conversation.
func (s *Service) CreateResearchChat(id, title string) (chat.Session, error) {
	_, st, err := s.researchChats(id)
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Create(title, "", "")
	return sess, chatErr(err)
}

// GetResearchChat returns a conversation and its answers' citations.
func (s *Service) GetResearchChat(id, chatID string) (chat.Session, map[string][]research.Citation, error) {
	nb, st, err := s.researchChats(id)
	if err != nil {
		return chat.Session{}, nil, err
	}
	sess, err := st.Get(chatID)
	if err != nil {
		return chat.Session{}, nil, chatErr(err)
	}
	cites, err := loadCitations(nb, chatID)
	return sess, cites, err
}

// UpdateResearchChat renames a conversation or sets its model.
func (s *Service) UpdateResearchChat(id, chatID string, p ChatPatch) (chat.Session, error) {
	model, effort := "", ""
	if p.Model != nil {
		model = *p.Model
	}
	if p.ThinkingEffort != nil {
		effort = *p.ThinkingEffort
	}
	if err := s.checkChatSettings(model, effort); err != nil {
		return chat.Session{}, err
	}
	_, st, err := s.researchChats(id)
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Update(chatID, func(sess *chat.Session) error {
		applyPatch(sess, p)
		return nil
	})
	return sess, chatErr(err)
}

// DeleteResearchChat deletes a conversation and its citations.
func (s *Service) DeleteResearchChat(id, chatID string) error {
	nb, st, err := s.researchChats(id)
	if err != nil {
		return err
	}
	if err := st.Delete(chatID); err != nil {
		return chatErr(err)
	}
	if err := os.Remove(citationsPath(nb, chatID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete citations: %w", err)
	}
	return nil
}

// researchBusy reports whether an answer is streaming in a conversation.
func (s *Service) researchBusy(id, chatID string) bool {
	_, st, err := s.researchChats(id)
	if err != nil {
		return false
	}
	release, err := st.Acquire(chatID)
	if err != nil {
		return errors.Is(err, chat.ErrBusy)
	}
	release()
	return false
}

// SendResearch asks a question in a conversation: the sources and notes in
// context, the relabelled history and the question go to the model; the
// answer is stored with the citations it made.
func (s *Service) SendResearch(ctx context.Context, id, chatID, text string, onDelta func(string),
) (ResearchReply, error) {
	if err := chat.CheckMessage(text, nil); err != nil {
		return ResearchReply{}, chatErr(err)
	}
	nb, st, err := s.researchChats(id)
	if err != nil {
		return ResearchReply{}, err
	}
	release, err := st.Acquire(chatID)
	if err != nil {
		return ResearchReply{}, chatErr(err)
	}
	defer release()
	sess, err := st.Get(chatID)
	if err != nil {
		return ResearchReply{}, chatErr(err)
	}
	settings := s.researchChatSettings(nb, sess)
	if settings.Model == "" {
		return ResearchReply{}, invalidf("no model set; set one for this notebook or defaults.model in the config")
	}
	alias, err := s.Config.ResolveModel(settings.Model)
	if err != nil {
		return ResearchReply{}, &InvalidError{err.Error()}
	}
	cites, err := loadCitations(nb, chatID)
	if err != nil {
		return ResearchReply{}, err
	}
	items, skipped := s.researchItems(ctx, nb, false)
	built := research.Build(nb.Manifest.Name, nb.Manifest.Description, items, relabelled(sess.Messages, cites), text,
		budget(settings))
	user := chat.Message{
		ID: chat.NewMessageID(), Role: chat.RoleUser, Content: text, CreatedAt: nowRFC3339(),
		Tokens: extract.EstimateTokens(text),
	}
	reply, runErr := s.researchAnswer(ctx, alias, settings, built, onDelta)
	out := ResearchReply{ChatReply: ChatReply{User: user, Assistant: reply}, Skipped: skipped}
	out.Citations = research.ParseCitations(reply.Content, built.Items)
	if len(skipped) > 0 {
		reply.Content = "_Skipped (could not read): " + strings.Join(skipped, ", ") + "_\n\n" + reply.Content
		out.Assistant = reply
	}
	if err := s.storeResearchTurn(nb, st, chatID, user, reply, out.Citations); err != nil {
		return out, errors.Join(runErr, err)
	}
	auditErr := s.auditMessage(KindResearchMessage, id+"/"+chatID, user, reply, settings)
	return out, errors.Join(runErr, auditErr)
}

// researchChatSettings resolves app, notebook and conversation settings.
func (s *Service) researchChatSettings(nb *research.Notebook, sess chat.Session) config.RunSettings {
	return config.Resolve(
		config.Layer{Name: layerApp, Settings: s.Config.Defaults},
		config.Layer{Name: "research", Settings: nb.Manifest.Settings},
		config.Layer{Name: "conversation", Settings: config.RunSettings{
			Model: sess.Model, ThinkingEffort: sess.ThinkingEffort,
		}},
	).RunSettings
}

func (s *Service) researchAnswer(ctx context.Context, alias config.Alias, settings config.RunSettings,
	built research.Built, onDelta func(string),
) (chat.Message, error) {
	reply := chat.Message{ID: chat.NewMessageID(), Role: chat.RoleAssistant, CreatedAt: nowRFC3339()}
	model, err := s.aliasModel(alias)
	if err != nil {
		reply.Stopped, reply.Error = true, errModelSetup
		return reply, err
	}
	req := llmRequest(settings, built)
	res, text, err := streamText(ctx, model, req, 0, onDelta)
	reply.Model, reply.Content, reply.Tokens = res.Target, text, res.Usage.CompletionTokens
	return reply, finishReply(&reply, err)
}

func (s *Service) storeResearchTurn(nb *research.Notebook, st *chat.Store, chatID string, user, reply chat.Message,
	cites []research.Citation,
) error {
	if _, err := st.Update(chatID, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, user, reply)
		sess.AutoTitle()
		return nil
	}); err != nil {
		return chatErr(err)
	}
	if len(cites) == 0 {
		return nil
	}
	all, err := loadCitations(nb, chatID)
	if err != nil {
		return err
	}
	all[reply.ID] = cites
	return research.SaveCitations(citationsPath(nb, chatID), all)
}

// relabelled is the history with each answer's citation numbers replaced by
// labels, so the next question's numbers cannot be confused with them.
func relabelled(msgs []chat.Message, cites map[string][]research.Citation) []chat.Message {
	out := make([]chat.Message, len(msgs))
	for i, m := range msgs {
		if m.Role == chat.RoleAssistant {
			m.Content = research.Relabel(m.Content, cites[m.ID])
		}
		out[i] = m
	}
	return out
}

func citationsPath(nb *research.Notebook, chatID string) string {
	return filepath.Join(nb.Dir, research.ChatsDir, chatID+".citations.json")
}

func loadCitations(nb *research.Notebook, chatID string) (map[string][]research.Citation, error) {
	if !chat.ValidID(chatID) {
		return nil, fmt.Errorf("conversation %q: %w", chatID, ErrNotFound)
	}
	return research.LoadCitations(citationsPath(nb, chatID))
}

// NoteFromMessage keeps an answer as a note: its question as the title, its
// citations as labels and a references list.
func (s *Service) NoteFromMessage(id, chatID, messageID string) (research.Note, error) {
	sess, cites, err := s.GetResearchChat(id, chatID)
	if err != nil {
		return research.Note{}, err
	}
	question := ""
	for _, m := range sess.Messages {
		if m.ID != messageID || m.Role != chat.RoleAssistant {
			if m.Role == chat.RoleUser {
				question = m.Content
			}
			continue
		}
		text := research.Relabel(m.Content, cites[m.ID])
		if refs := referenceList(cites[m.ID]); refs != "" {
			text += "\n\nReferences:\n" + refs
		}
		return s.SaveResearchNote(id, research.Note{
			Title: noteTitle(question), Text: text, Origin: research.OriginAI, Include: true,
		})
	}
	return research.Note{}, fmt.Errorf("message %s: %w", messageID, ErrNotFound)
}

func referenceList(cites []research.Citation) string {
	var sb strings.Builder
	for _, c := range cites {
		line := "- " + c.Label
		if c.Path != "" {
			line += ": " + c.Path
		}
		if c.Level != "" {
			line += " (" + c.Level + ")"
		}
		sb.WriteString(line + "\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func noteTitle(question string) string {
	q := strings.Join(strings.Fields(question), " ")
	if r := []rune(q); len(r) > 80 {
		return string(r[:79]) + "…"
	}
	if q == "" {
		return "Saved answer"
	}
	return q
}

func llmRequest(settings config.RunSettings, built research.Built) llm.Request {
	return llm.Request{
		Model: settings.Model, MaxTokens: settings.MaxOutputTokens, Temperature: settings.Temperature,
		ReasoningEffort: settings.ThinkingEffort, Messages: built.Messages,
	}
}
