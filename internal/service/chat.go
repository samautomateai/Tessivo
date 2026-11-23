package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
)

// ErrConflict marks a request that clashes with work in progress (HTTP 409).
var ErrConflict = errors.New("conflict")

// ChatInput creates a session.
type ChatInput struct {
	Title          string `json:"title"`
	Model          string `json:"model"`
	ThinkingEffort string `json:"thinkingEffort"`
}

// ChatPatch changes a session; nil fields stay as they are.
type ChatPatch struct {
	Title          *string   `json:"title"`
	Pinned         *bool     `json:"pinned"`
	Archived       *bool     `json:"archived"`
	Model          *string   `json:"model"`
	ThinkingEffort *string   `json:"thinkingEffort"`
	MCPServers     *[]string `json:"mcpServers"` // servers whose tools the chat may use (configured ones only)
}

// ChatDirName is the chat store folder next to the notebooks workspace.
const ChatDirName = "chat"

func (s *Service) chatStore() (*chat.Store, error) {
	dir := s.ChatDir
	if dir == "" {
		if s.Workspace == "" {
			return nil, errors.New("chat: no workspace configured")
		}
		dir = filepath.Join(filepath.Dir(filepath.Clean(s.Workspace)), ChatDirName)
	}
	st, err := chat.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	return st, nil
}

// chatErr maps chat package errors onto the service's error kinds.
func chatErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, chat.ErrNotFound):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, chat.ErrInvalid):
		return &InvalidError{err.Error()}
	case errors.Is(err, chat.ErrBusy):
		return fmt.Errorf("%w: %w", ErrConflict, err)
	default:
		return err
	}
}

// checkChatSettings validates a model name and thinking effort ("" means default).
func (s *Service) checkChatSettings(model, effort string) error {
	if model != "" {
		if _, err := s.cfg().ResolveModel(model); err != nil {
			return &InvalidError{err.Error()}
		}
	}
	if err := (config.RunSettings{ThinkingEffort: effort}).Validate(); err != nil {
		return &InvalidError{err.Error()}
	}
	return nil
}

// ListChats lists sessions matching query, active or archived.
func (s *Service) ListChats(query string, archived bool) ([]chat.Summary, error) {
	st, err := s.chatStore()
	if err != nil {
		return nil, err
	}
	out, err := st.List(chat.Filter{Query: query, Archived: archived})
	return out, chatErr(err)
}

// CreateChat starts a session.
func (s *Service) CreateChat(in ChatInput) (chat.Session, error) {
	if err := s.checkChatSettings(in.Model, in.ThinkingEffort); err != nil {
		return chat.Session{}, err
	}
	st, err := s.chatStore()
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Create(in.Title, in.Model, in.ThinkingEffort)
	return sess, chatErr(err)
}

// GetChat returns a session with its messages.
func (s *Service) GetChat(id string) (chat.Session, error) {
	st, err := s.chatStore()
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Get(id)
	return sess, chatErr(err)
}

// UpdateChat renames, pins, archives or changes a session's model settings.
func (s *Service) UpdateChat(id string, p ChatPatch) (chat.Session, error) {
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
	if p.Title != nil {
		if err := chat.CheckTitle(*p.Title); err != nil {
			return chat.Session{}, chatErr(err)
		}
	}
	if p.MCPServers != nil {
		if err := s.checkMCPNames(*p.MCPServers); err != nil {
			return chat.Session{}, err
		}
	}
	st, err := s.chatStore()
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Update(id, func(sess *chat.Session) error {
		applyPatch(sess, p)
		return nil
	})
	return sess, chatErr(err)
}

func applyPatch(sess *chat.Session, p ChatPatch) {
	if p.MCPServers != nil {
		sess.MCPServers = slices.Compact(slices.Clone(*p.MCPServers))
	}
	if p.Title != nil {
		sess.Title = strings.TrimSpace(*p.Title)
	}
	if p.Pinned != nil {
		sess.Pinned = *p.Pinned
	}
	if p.Archived != nil {
		sess.Archived = *p.Archived
	}
	if p.Model != nil {
		sess.Model = *p.Model
	}
	if p.ThinkingEffort != nil {
		sess.ThinkingEffort = *p.ThinkingEffort
	}
}

// DeleteChat removes a session.
func (s *Service) DeleteChat(id string) error {
	st, err := s.chatStore()
	if err != nil {
		return err
	}
	return chatErr(st.Delete(id))
}

// ChatMemory returns the global chat memory.
func (s *Service) ChatMemory() ([]string, error) {
	st, err := s.chatStore()
	if err != nil {
		return nil, err
	}
	m, err := st.Memory()
	return m, chatErr(err)
}

// SetChatMemory replaces the global chat memory.
func (s *Service) SetChatMemory(facts []string) ([]string, error) {
	st, err := s.chatStore()
	if err != nil {
		return nil, err
	}
	m, err := st.SetMemory(facts)
	return m, chatErr(err)
}

// chatModelSettings resolves the session's model and thinking effort over the
// app defaults.
func (s *Service) chatModelSettings(sess chat.Session) config.RunSettings {
	return config.Resolve(
		config.Layer{Name: layerApp, Settings: s.cfg().Defaults},
		config.Layer{Name: "chat", Settings: config.RunSettings{
			Model: sess.Model, ThinkingEffort: sess.ThinkingEffort,
		}},
	).RunSettings
}

// stopped reports whether err means the caller stopped the reply.
func stopped(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
