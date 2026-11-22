package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/extract"
	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/template"
	"github.com/samautomateai/tessivo/internal/worktype"
)

// ChatPromptName is the prompt a chat turned into a notebook carries.
const ChatPromptName = "from-chat"

// Document kinds for text the service passes to the model directly.
const (
	docMarkdown = "markdown"
	docText     = "text"
)

// minTemplateScore is how many distinct words a chat must share with a
// template's name and description before the template is suggested.
const minTemplateScore = 2

// RepeatablePlan is what "Make this repeatable" would create from a chat.
type RepeatablePlan struct {
	Name         string            `json:"name"`
	WorkType     string            `json:"workType"`
	TemplateID   string            `json:"templateId,omitempty"`
	TemplateName string            `json:"templateName,omitempty"`
	Prompt       string            `json:"prompt"`
	Context      []string          `json:"context"` // absolute paths to add as notebook context
	Skipped      []string          `json:"skipped"` // attachments that cannot become context, with why
	labels       map[string]string // display labels for paths copied from other notebooks
}

// RepeatableInput overrides the plan; empty fields keep the suggestion.
// TemplateID "none" creates a plain notebook even when a template is suggested.
type RepeatableInput struct {
	Name       string `json:"name"`
	WorkType   string `json:"workType"`
	TemplateID string `json:"templateId"`
	Prompt     string `json:"prompt"`
}

// PlanRepeatable suggests a notebook for a chat: work type, template, the
// final prompt and the context to carry over.
func (s *Service) PlanRepeatable(chatID string) (RepeatablePlan, error) {
	sess, err := s.GetChat(chatID)
	if err != nil {
		return RepeatablePlan{}, err
	}
	prompt := chat.LastUserMessage(sess)
	if prompt == "" {
		return RepeatablePlan{}, invalidf("this chat has no messages to repeat yet")
	}
	plan := RepeatablePlan{
		Name: sess.Title, WorkType: chat.SuggestWorkType(sess), Prompt: prompt,
		Context: []string{}, Skipped: []string{}, labels: map[string]string{},
	}
	if plan.Name == "" {
		plan.Name = "From chat"
	}
	if t, ok := s.suggestTemplate(plan.WorkType, chat.UserText(sess)); ok {
		plan.TemplateID, plan.TemplateName = t.ID, t.Name
	}
	for _, a := range sess.Attachments() {
		paths, why := s.contextPaths(a, plan.labels)
		plan.Context = append(plan.Context, paths...)
		if why != "" {
			plan.Skipped = append(plan.Skipped, attachmentName(a)+": "+why)
		}
	}
	return plan, nil
}

func attachmentName(a chat.Attachment) string {
	if a.Label != "" {
		return a.Label
	}
	return a.Ref
}

// contextPaths turns an attachment into notebook context paths, or says why not.
func (s *Service) contextPaths(a chat.Attachment, labels map[string]string) ([]string, string) {
	switch a.Kind {
	case chat.AttachPath:
		return []string{a.Ref}, ""
	case chat.AttachNotebook:
		nb, err := s.OpenNotebook(a.Ref)
		if err != nil {
			return nil, "notebook not found"
		}
		var out []string
		for _, c := range nb.Manifest.Context {
			if c.Enabled && !c.IsMCP() { // MCP resources are not files to copy
				p := nb.ContextPath(c)
				labels[p] = c.Label
				out = append(out, p)
			}
		}
		return out, ""
	default:
		return nil, "saved outputs are not carried over; attach the sources instead"
	}
}

// suggestTemplate returns the template of workType sharing the most words
// with the chat, if it clears minTemplateScore.
func (s *Service) suggestTemplate(workType, text string) (template.Template, bool) {
	list, err := s.Templates.List()
	if err != nil {
		return template.Template{}, false
	}
	words := map[string]bool{}
	for _, w := range chat.Words(text) {
		if len(w) >= 4 {
			words[w] = true
		}
	}
	var best template.Template
	bestScore := 0
	for _, t := range list {
		if t.WorkType != workType {
			continue
		}
		score, seen := 0, map[string]bool{}
		for _, w := range chat.Words(t.Name + " " + t.Description) {
			if len(w) >= 4 && words[w] && !seen[w] {
				seen[w] = true
				score++
			}
		}
		if score > bestScore {
			best, bestScore = t, score
		}
	}
	return best, bestScore >= minTemplateScore
}

// MakeRepeatable creates a notebook from a chat (CHAT-6): the suggested or
// chosen work type and template, the chat's context and its final prompt.
func (s *Service) MakeRepeatable(chatID string, in RepeatableInput) (NotebookInfo, error) {
	plan, err := s.PlanRepeatable(chatID)
	if err != nil {
		return NotebookInfo{}, err
	}
	applyRepeatableInput(&plan, in)
	if _, err := worktype.Get(plan.WorkType); err != nil {
		return NotebookInfo{}, &InvalidError{err.Error()}
	}
	info, err := s.createForPlan(plan)
	if err != nil {
		return NotebookInfo{}, err
	}
	nb, err := s.OpenNotebook(info.ID)
	if err != nil {
		return NotebookInfo{}, err
	}
	for _, p := range plan.Context {
		if _, err := nb.AddContext(p); err != nil {
			return NotebookInfo{}, &InvalidError{fmt.Sprintf("context %s: %v", filepath.Base(p), err)}
		}
		if l := plan.labels[p]; l != "" {
			nb.Manifest.Context[len(nb.Manifest.Context)-1].Label = l
		}
	}
	if err := nb.Save(); err != nil {
		return NotebookInfo{}, err
	}
	if err := nb.SavePrompt(notebook.Prompt{Name: ChatPromptName, Text: plan.Prompt}); err != nil {
		return NotebookInfo{}, err
	}
	return describe(info.ID, nb, false), nil
}

func applyRepeatableInput(plan *RepeatablePlan, in RepeatableInput) {
	if strings.TrimSpace(in.Name) != "" {
		plan.Name = strings.TrimSpace(in.Name)
	}
	if in.WorkType != "" && in.WorkType != plan.WorkType {
		plan.WorkType = in.WorkType
		plan.TemplateID = "" // the suggestion was for the other work type
	}
	switch in.TemplateID {
	case "":
	case "none":
		plan.TemplateID = ""
	default:
		plan.TemplateID = in.TemplateID
	}
	if strings.TrimSpace(in.Prompt) != "" {
		plan.Prompt = in.Prompt
	}
}

func (s *Service) createForPlan(plan RepeatablePlan) (NotebookInfo, error) {
	if plan.TemplateID == "" {
		return s.CreateNotebook(plan.Name, plan.WorkType)
	}
	t, err := s.Templates.Get(plan.TemplateID)
	if err != nil {
		return NotebookInfo{}, s.templateErr(err)
	}
	if t.WorkType != plan.WorkType {
		return NotebookInfo{}, invalidf("template %s is a %s template, not %s", t.ID, t.WorkType, plan.WorkType)
	}
	return s.CreateFromTemplate(t.ID, plan.Name)
}

// chatOutputs serializes creating the per-work-type chat output notebooks.
var chatOutputs sync.Mutex

// chatOutputNotebook returns the notebook that holds chat replies saved as
// outputs of workType ("chat-analyze" and so on), creating it on first use.
func (s *Service) chatOutputNotebook(workType string) (*notebook.Notebook, error) {
	wt, err := worktype.Get(workType)
	if err != nil {
		return nil, &InvalidError{err.Error()}
	}
	id := "chat-" + wt.ID
	chatOutputs.Lock()
	defer chatOutputs.Unlock()
	nb, err := s.OpenNotebook(id)
	if errors.Is(err, ErrNotFound) {
		if _, err := s.CreateNotebook("Chat "+wt.ID, wt.ID); err != nil {
			return nil, err
		}
		nb, err = s.OpenNotebook(id)
	}
	if err != nil {
		return nil, err
	}
	if nb.Manifest.WorkType != wt.ID {
		return nil, invalidf("notebook %s exists but is a %s notebook", id, nb.Manifest.WorkType)
	}
	return nb, nil
}

// structurePrompt asks the model to restate a reply in the work type's shape.
const structurePrompt = "Restructure the attached chat reply into this work type's output. Keep its facts and " +
	"conclusions; do not add facts that are not in the reply or the question. Use the question for the title."

// ChatOutput is a chat reply saved to the outputs library.
type ChatOutput struct {
	NotebookID string `json:"notebookId"`
	OutputID   string `json:"outputId"`
	RunID      string `json:"runId"`
	Title      string `json:"title"`
	Markdown   string `json:"markdown"`
}

// SaveChatReply saves an assistant reply to the outputs library as a validated
// output of workType (CHAT-7). The model restructures the reply; the result is
// validated against the work type's schema before it is saved.
func (s *Service) SaveChatReply(ctx context.Context, chatID, messageID, workType string) (ChatOutput, error) {
	sess, err := s.GetChat(chatID)
	if err != nil {
		return ChatOutput{}, err
	}
	msg, ok := sess.Find(messageID)
	if !ok {
		return ChatOutput{}, fmt.Errorf("message %s: %w", messageID, ErrNotFound)
	}
	if msg.Role != chat.RoleAssistant || strings.TrimSpace(msg.Content) == "" {
		return ChatOutput{}, invalidf("only non-empty replies can be saved as outputs")
	}
	nb, err := s.chatOutputNotebook(workType)
	if err != nil {
		return ChatOutput{}, err
	}
	extra := []extract.Document{{Source: "chat reply", Kind: docMarkdown, Text: msg.Content}}
	if q := sess.QuestionBefore(messageID); q != "" {
		extra = append([]extract.Document{{Source: "question", Kind: docText, Text: q}}, extra...)
	}
	model := sess.Model
	if model == "" {
		model = s.cfg().Defaults.Model
	}
	req := RunRequest{Notebook: nb, Prompt: structurePrompt, Extra: extra, Save: true}
	req.Overrides.Model = model
	out, err := s.Run(ctx, req, nil)
	if err != nil {
		return ChatOutput{}, err
	}
	return ChatOutput{
		NotebookID: filepath.Base(nb.Dir), OutputID: out.OutputID, RunID: out.RunID,
		Title: titleOf(out.Result.Data), Markdown: out.Markdown,
	}, nil
}
