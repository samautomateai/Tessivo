package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/executor"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/llm"
	"github.com/samautomateai/tessivo/internal/notebook"
)

// KindThreadMessage is the audit entry for one reply in an output's thread.
const KindThreadMessage = "thread.message"

// ThreadReply is a stored thread turn plus the suggestions the reply made.
type ThreadReply struct {
	ChatReply
	Suggestions []notebook.Suggestion `json:"suggestions"`
}

// outputRef is an output with the records a thread needs.
type outputRef struct {
	nb  *notebook.Notebook
	out notebook.Output
	md  string
	run notebook.Run
}

// loadOutput opens a notebook's output and the run that produced it.
func (s *Service) loadOutput(notebookID, outputID string) (outputRef, error) {
	nb, err := s.openNotebookRef(notebookID)
	if err != nil {
		return outputRef{}, err
	}
	if !notebook.ValidID(outputID) {
		return outputRef{}, invalidf("output id %q is not valid", outputID)
	}
	o, md, err := nb.GetOutput(outputID)
	if errors.Is(err, fs.ErrNotExist) {
		return outputRef{}, fmt.Errorf("output %s: %w", outputID, ErrNotFound)
	}
	if err != nil {
		return outputRef{}, fmt.Errorf("output %s: %w", outputID, err)
	}
	run, err := nb.LoadRun(o.RunID)
	if errors.Is(err, fs.ErrNotExist) {
		return outputRef{}, fmt.Errorf("run %s of output %s: %w", o.RunID, outputID, ErrNotFound)
	}
	if err != nil {
		return outputRef{}, fmt.Errorf("run of output %s: %w", outputID, err)
	}
	return outputRef{nb: nb, out: o, md: md, run: run}, nil
}

func (s *Service) threadStore(nb *notebook.Notebook) (*chat.Store, error) {
	st, err := chat.OpenKeyed(filepath.Join(nb.Dir, notebook.ThreadsDir), notebook.ValidID)
	if err != nil {
		return nil, fmt.Errorf("thread: %w", err)
	}
	return st, nil
}

// GetThread returns an output's thread; an output without one gets an empty
// session (nothing is written).
func (s *Service) GetThread(notebookID, outputID string) (chat.Session, error) {
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return chat.Session{}, err
	}
	st, err := s.threadStore(ref.nb)
	if err != nil {
		return chat.Session{}, err
	}
	sess, err := st.Get(outputID)
	if errors.Is(err, chat.ErrNotFound) {
		return chat.Session{ID: outputID, Messages: []chat.Message{}}, nil
	}
	return sess, chatErr(err)
}

// SetThreadSettings sets the thread's model and thinking effort ("" uses the
// notebook's settings).
func (s *Service) SetThreadSettings(notebookID, outputID, model, effort string) (chat.Session, error) {
	if err := s.checkChatSettings(model, effort); err != nil {
		return chat.Session{}, err
	}
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return chat.Session{}, err
	}
	st, err := s.threadStore(ref.nb)
	if err != nil {
		return chat.Session{}, err
	}
	if _, err := st.Ensure(outputID, "", ""); err != nil {
		return chat.Session{}, chatErr(err)
	}
	sess, err := st.Update(outputID, func(sess *chat.Session) error {
		sess.Model, sess.ThinkingEffort = model, effort
		return nil
	})
	return sess, chatErr(err)
}

// CheckThreadSend validates a message without sending it.
func (s *Service) CheckThreadSend(notebookID, outputID string, in ChatSend) error {
	if err := chat.CheckMessage(in.Text, in.Attachments); err != nil {
		return chatErr(err)
	}
	_, err := s.loadOutput(notebookID, outputID)
	return err
}

// SendThread stores the user's message, streams the reply through onDelta and
// stores it. Suggestions the model makes are stored open and reported through
// onSuggestion. Cancelling ctx stops the reply, as in chat.
func (s *Service) SendThread(ctx context.Context, notebookID, outputID string, in ChatSend,
	onDelta func(string), onSuggestion func(notebook.Suggestion),
) (ThreadReply, error) {
	if err := chat.CheckMessage(in.Text, in.Attachments); err != nil {
		return ThreadReply{}, chatErr(err)
	}
	ref, err := s.loadOutput(notebookID, outputID)
	if err != nil {
		return ThreadReply{}, err
	}
	st, err := s.threadStore(ref.nb)
	if err != nil {
		return ThreadReply{}, err
	}
	release, err := st.Acquire(outputID)
	if err != nil {
		return ThreadReply{}, chatErr(err)
	}
	defer release()
	sess, user, req, alias, err := s.prepareThread(st, ref, in)
	if err != nil {
		return ThreadReply{}, err
	}
	reply, sugs, runErr := s.threadReply(ctx, ref, alias, req, onDelta, onSuggestion)
	out := ThreadReply{ChatReply: ChatReply{User: user, Assistant: reply}, Suggestions: sugs}
	if _, err := st.Update(outputID, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, reply)
		return nil
	}); err != nil {
		return out, errors.Join(runErr, chatErr(err))
	}
	auditErr := s.auditMessage(KindThreadMessage, notebookID+"/"+outputID, user, reply, s.threadSettings(ref.nb, sess))
	return out, errors.Join(runErr, auditErr)
}

// prepareThread stores the user's message and builds the first request.
func (s *Service) prepareThread(st *chat.Store, ref outputRef, in ChatSend,
) (chat.Session, chat.Message, llm.Request, config.Alias, error) {
	fail := func(err error) (chat.Session, chat.Message, llm.Request, config.Alias, error) {
		return chat.Session{}, chat.Message{}, llm.Request{}, config.Alias{}, err
	}
	sess, err := st.Ensure(ref.out.ID, "", "")
	if err != nil {
		return fail(chatErr(err))
	}
	settings := s.threadSettings(ref.nb, sess)
	if settings.Model == "" {
		return fail(invalidf("no model set; choose one for this thread or set defaults.model in the config"))
	}
	alias, err := s.Config.ResolveModel(settings.Model)
	if err != nil {
		return fail(&InvalidError{err.Error()})
	}
	atts, extra, err := s.resolveAttachments(in.Attachments)
	if err != nil {
		return fail(err)
	}
	docs, err := threadDocs(ref)
	if err != nil {
		return fail(err)
	}
	user := chat.Message{
		ID: chat.NewMessageID(), Role: chat.RoleUser, Content: in.Text, CreatedAt: nowRFC3339(),
		Attachments: atts, Tokens: extract.EstimateTokens(in.Text),
	}
	sess, err = st.Update(ref.out.ID, func(sess *chat.Session) error {
		sess.Messages = append(sess.Messages, user)
		return nil
	})
	if err != nil {
		return fail(chatErr(err))
	}
	docs = append(append(docs, extra...), s.sessionDocs(sess, user.ID)...)
	req := llm.Request{
		Model: settings.Model, MaxTokens: settings.MaxOutputTokens, Temperature: settings.Temperature,
		ReasoningEffort: settings.ThinkingEffort, Tools: suggestionTools,
		Messages: chat.BuildThread(sess, ref.nb.Manifest.Memory, docs, budget(settings)),
	}
	return sess, user, req, alias, nil
}

// threadSettings resolves app, notebook and thread settings.
func (s *Service) threadSettings(nb *notebook.Notebook, sess chat.Session) config.RunSettings {
	return config.Resolve(
		config.Layer{Name: layerApp, Settings: s.Config.Defaults},
		config.Layer{Name: layerNotebook, Settings: nb.Manifest.Settings},
		config.Layer{Name: "thread", Settings: config.RunSettings{
			Model: sess.Model, ThinkingEffort: sess.ThinkingEffort,
		}},
	).RunSettings
}

// threadDocs are the output, the prompt that produced it and the notebook's
// enabled context, in that order (the budget trims larger sources first).
func threadDocs(ref outputRef) ([]extract.Document, error) {
	text := ref.md
	if text == "" {
		text = string(ref.out.Data)
	}
	docs := []extract.Document{
		{Source: "output", Kind: "markdown", Text: text},
		{Source: "prompt", Kind: "text", Text: ref.run.Prompt},
	}
	nbDocs, err := notebookDocs(ref.nb)
	if err != nil {
		return nil, err
	}
	return append(docs, nbDocs...), nil
}

// threadReply runs the model with the suggestion tools, records any
// suggestions, and makes one more call for the text answer. A model that
// rejects tools, or asks for more than the tool limits allow, is asked again
// without tools.
func (s *Service) threadReply(ctx context.Context, ref outputRef, alias config.Alias, req llm.Request,
	onDelta func(string), onSuggestion func(notebook.Suggestion),
) (chat.Message, []notebook.Suggestion, error) {
	reply := chat.Message{ID: chat.NewMessageID(), Role: chat.RoleAssistant, CreatedAt: nowRFC3339()}
	model, err := s.aliasModel(alias)
	if err != nil {
		reply.Stopped, reply.Error = true, "model setup failed"
		return reply, nil, err
	}
	res, text, err := streamText(ctx, model, req, 0, onDelta)
	if errors.Is(err, llm.ErrToolsUnsupported) || errors.Is(err, llm.ErrToolCallLimit) {
		req.Tools = nil
		res, text, err = streamText(ctx, model, req, 0, onDelta)
	}
	var sugs []notebook.Suggestion
	if err == nil && len(res.ToolCalls) > 0 {
		var results []llm.Message
		sugs, results, _ = s.recordSuggestions(ref, reply.ID, res.ToolCalls, onSuggestion) // results say why
		text, err = followUp(ctx, model, req, &res, results, text, onDelta)
	}
	reply.Model, reply.Content, reply.Tokens = res.Target, text, res.Usage.CompletionTokens
	return reply, sugs, finishReply(&reply, err)
}

// followUp sends the tool results back for the text answer. The tools stay
// defined (some providers require them once the history holds tool calls)
// but tool_choice "none" forbids new calls, so there is one extra round only.
// The rounds' texts are joined by a blank line; res gains the usage.
func followUp(ctx context.Context, model executor.Model, req llm.Request, res *llm.Result, results []llm.Message,
	text string, onDelta func(string),
) (string, error) {
	follow := req
	follow.ToolChoice = "none"
	follow.Messages = append(slices.Clone(req.Messages),
		llm.Message{Role: llm.RoleAssistant, Content: res.Content, ToolCalls: res.ToolCalls})
	follow.Messages = append(follow.Messages, results...)
	sep := ""
	if strings.TrimSpace(text) != "" {
		sep = "\n\n"
	}
	started := false
	more, moreText, err := streamText(ctx, model, follow, len(text)+len(sep), func(d string) {
		if !started {
			d, started = sep+d, true
		}
		if onDelta != nil {
			onDelta(d)
		}
	})
	res.Usage.CompletionTokens += more.Usage.CompletionTokens
	if more.Target != "" {
		res.Target = more.Target
	}
	if moreText == "" {
		return text, err
	}
	return text + sep + moreText, err
}

// Settings layer names.
const (
	layerApp      = "app"
	layerNotebook = "notebook"
)

// openNotebookRef opens a workspace notebook by ID, or a folder the caller
// listed in NotebookDirs (the CLI does, for notebooks outside the workspace;
// the HTTP API never does, so clients cannot name arbitrary folders).
func (s *Service) openNotebookRef(ref string) (*notebook.Notebook, error) {
	for _, d := range s.NotebookDirs {
		if filepath.Clean(d) == filepath.Clean(ref) {
			return notebook.Open(d)
		}
	}
	return s.OpenNotebook(ref)
}
