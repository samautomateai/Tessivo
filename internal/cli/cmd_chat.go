package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/samautomateai/tessivo/internal/chat"
	"github.com/samautomateai/tessivo/internal/service"
)

// chatHelp is the chat section of 'tess chat -h'.
const chatHelp = `tess chat [--session ID] [--model M] [--thinking E] [--attach PATH]... [message]
    one message, piped text, or an interactive session when neither is given (/quit to leave)
tess chat list [--archived] [--search TEXT] | show <id> | rename <id> <title> | pin|unpin|archive|unarchive <id>
tess chat delete <id>
tess chat memory list|add|remove [text|number]   global memory shared by every chat
tess chat repeat <id> [--name N] [--type T] [--template ID|none]   turn a chat into a notebook
tess chat save <id> <message-id|last> --type T   save a reply as a validated output`

var chatCommands = map[string]func(context.Context, *service.Service, []string, Env) error{
	subList: chatList, subShow: chatShow, "rename": chatRename, subDelete: chatDelete,
	"pin": chatFlag, "unpin": chatFlag, "archive": chatFlag, "unarchive": chatFlag,
	"memory": chatMemory, "repeat": chatRepeat, subSave: chatSave,
}

func cmdChat(ctx context.Context, args []string, env Env) error {
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	if len(args) > 0 {
		if sub, ok := chatCommands[args[0]]; ok {
			return cliErr(sub(ctx, svc, args, env))
		}
	}
	return cliErr(chatSend(ctx, svc, args, env))
}

// cliErr turns service bad-request errors into usage errors (exit 2).
func cliErr(err error) error {
	if err != nil && service.IsInvalid(err) {
		return usageError{err.Error()}
	}
	return err
}

func chatSend(ctx context.Context, svc *service.Service, args []string, env Env) error {
	fs := newFlags("chat", env)
	fs.Usage = func() { _, _ = fmt.Fprintln(env.Stderr, chatHelp) }
	session := fs.String("session", "", "continue this chat session")
	model := fs.String("model", "", "model alias for a new session")
	thinking := fs.String("thinking", "", "thinking effort for a new session: off, low, medium, high")
	var attach multiFlag
	fs.Var(&attach, "attach", "absolute path to a file or folder (repeatable)")
	rest, err := parse(fs, args)
	if err != nil {
		return err
	}
	atts, err := attachments(attach)
	if err != nil {
		return err
	}
	text := strings.Join(rest, " ")
	if text != "" {
		if err := chat.CheckMessage(text, atts); err != nil {
			return usageError{err.Error()}
		}
	}
	id := *session
	if id == "" {
		sess, err := svc.CreateChat(service.ChatInput{Model: *model, ThinkingEffort: *thinking})
		if err != nil {
			return err
		}
		id = sess.ID
		_, _ = fmt.Fprintf(env.Stderr, "session %s\n", id)
	}
	switch {
	case text != "":
		return sendOne(ctx, svc, id, text, atts, env)
	case !env.StdinIsTTY && env.Stdin != nil:
		b, err := io.ReadAll(io.LimitReader(env.Stdin, chat.MaxMessageBytes+1))
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		return sendOne(ctx, svc, id, string(b), atts, env)
	default:
		return chatLoop(ctx, svc, id, atts, env)
	}
}

func attachments(paths []string) ([]chat.Attachment, error) {
	out := make([]chat.Attachment, 0, len(paths))
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, usagef("attach %s: %v", p, err)
		}
		out = append(out, chat.Attachment{Kind: chat.AttachPath, Ref: abs})
	}
	return out, nil
}

func sendOne(ctx context.Context, svc *service.Service, id, text string, atts []chat.Attachment, env Env) error {
	var werr error
	streamed := false
	reply, err := svc.SendChat(ctx, id, service.ChatSend{Text: text, Attachments: atts}, func(d string) {
		streamed = true
		if werr == nil {
			_, werr = io.WriteString(env.Stdout, d)
		}
	})
	if !streamed && werr == nil {
		_, werr = io.WriteString(env.Stdout, reply.Assistant.Content) // provider sent no deltas
	}
	if werr == nil && reply.Assistant.Content != "" {
		_, werr = io.WriteString(env.Stdout, "\n")
	}
	if err != nil {
		return err
	}
	if reply.Assistant.Stopped {
		_, _ = fmt.Fprintln(env.Stderr, "(stopped)")
	}
	return wrapWrite(werr)
}

// chatLoop reads one message per line until EOF or /quit. Attachments go with
// the first message only.
func chatLoop(ctx context.Context, svc *service.Service, id string, atts []chat.Attachment, env Env) error {
	sc := bufio.NewScanner(env.Stdin)
	sc.Buffer(make([]byte, 0, 4096), chat.MaxMessageBytes)
	for {
		_, _ = fmt.Fprint(env.Stderr, "> ")
		if !sc.Scan() {
			return wrapRead(sc.Err())
		}
		line := strings.TrimSpace(sc.Text())
		switch line {
		case "":
			continue
		case "/quit", "/exit":
			return nil
		}
		if err := sendOne(ctx, svc, id, line, atts, env); err != nil {
			if ctx.Err() != nil {
				return err
			}
			_, _ = fmt.Fprintf(env.Stderr, "tess: %v\n", err)
		}
		atts = nil
	}
}

func wrapRead(err error) error {
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	return nil
}

func chatList(_ context.Context, svc *service.Service, args []string, env Env) error {
	fs := newFlags("chat list", env)
	archived := fs.Bool("archived", false, "list archived chats")
	search := fs.String("search", "", "only chats whose title or messages contain TEXT")
	if _, err := parse(fs, args[1:]); err != nil {
		return err
	}
	list, err := svc.ListChats(*search, *archived)
	if err != nil {
		return err
	}
	for _, s := range list {
		pin := " "
		if s.Pinned {
			pin = "*"
		}
		if _, err := fmt.Fprintf(env.Stdout, "%s %s  %-40s  %3d msgs  %s\n", pin, s.ID, s.Title, s.MessageCount,
			s.UpdatedAt); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}

func chatShow(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) != 2 {
		return usagef("chat show: want <id>")
	}
	s, err := svc.GetChat(args[1])
	if err != nil {
		return err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", s.Title)
	for _, m := range s.Messages {
		fmt.Fprintf(&sb, "## %s · %s · %s\n", m.Role, m.ID, m.CreatedAt)
		for _, a := range m.Attachments {
			fmt.Fprintf(&sb, "[%s: %s, ≈%d tokens]\n", a.Kind, attachmentLabel(a), a.Tokens)
		}
		sb.WriteString(m.Content + "\n")
		if m.Stopped {
			fmt.Fprintf(&sb, "(stopped%s)\n", suffix(m.Error))
		}
		sb.WriteString("\n")
	}
	_, err = io.WriteString(env.Stdout, sb.String())
	return wrapWrite(err)
}

func attachmentLabel(a chat.Attachment) string {
	if a.Label != "" {
		return a.Label
	}
	return a.Ref
}

func suffix(s string) string {
	if s == "" {
		return ""
	}
	return ": " + s
}

func chatRename(_ context.Context, svc *service.Service, args []string, _ Env) error {
	if len(args) < 3 {
		return usagef("chat rename: want <id> <title>")
	}
	title := strings.Join(args[2:], " ")
	_, err := svc.UpdateChat(args[1], service.ChatPatch{Title: &title})
	return err
}

func chatFlag(_ context.Context, svc *service.Service, args []string, _ Env) error {
	if len(args) != 2 {
		return usagef("chat %s: want <id>", args[0])
	}
	on := !strings.HasPrefix(args[0], "un")
	var p service.ChatPatch
	if strings.HasSuffix(args[0], "pin") {
		p.Pinned = &on
	} else {
		p.Archived = &on
	}
	_, err := svc.UpdateChat(args[1], p)
	return err
}

func chatDelete(_ context.Context, svc *service.Service, args []string, _ Env) error {
	if len(args) != 2 {
		return usagef("chat delete: want <id>")
	}
	return svc.DeleteChat(args[1])
}

func chatMemory(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) < 2 {
		return usagef("chat memory: want list|add|remove [text|number]")
	}
	mem, err := svc.ChatMemory()
	if err != nil {
		return err
	}
	switch args[1] {
	case subList:
		for i, m := range mem {
			if _, err := fmt.Fprintf(env.Stdout, "%d. %s\n", i+1, m); err != nil {
				return wrapWrite(err)
			}
		}
		return nil
	case subAdd:
		text := strings.TrimSpace(strings.Join(args[2:], " "))
		if text == "" {
			return usagef("chat memory add: want text")
		}
		_, err = svc.SetChatMemory(append(mem, text))
	case subRemove:
		n, convErr := strconv.Atoi(strings.Join(args[2:], ""))
		if convErr != nil || n < 1 || n > len(mem) {
			return usagef("chat memory remove: want an item number from 'tess chat memory list'")
		}
		_, err = svc.SetChatMemory(append(mem[:n-1:n-1], mem[n:]...))
	default:
		return usagef("chat memory: unknown subcommand %q", args[1])
	}
	return err
}

func chatRepeat(_ context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) < 2 {
		return usagef("chat repeat: want <id> [--name N] [--type T] [--template ID|none]")
	}
	fs := newFlags("chat repeat", env)
	var in service.RepeatableInput
	fs.StringVar(&in.Name, "name", "", "notebook name (default: the chat title)")
	fs.StringVar(&in.WorkType, "type", "", "work type (default: suggested from the chat)")
	fs.StringVar(&in.TemplateID, "template", "", "template ID, or none (default: suggested)")
	if _, err := parse(fs, args[2:]); err != nil {
		return err
	}
	info, err := svc.MakeRepeatable(args[1], in)
	if err != nil {
		return err
	}
	tpl := ""
	if info.Template != "" {
		tpl = " from template " + info.Template
	}
	_, err = fmt.Fprintf(env.Stdout, "created notebook %s (%s%s) with %d context items and prompt %q\n",
		info.ID, info.WorkType, tpl, len(info.Context), service.ChatPromptName)
	return wrapWrite(err)
}

func chatSave(ctx context.Context, svc *service.Service, args []string, env Env) error {
	if len(args) < 3 {
		return usagef("chat save: want <id> <message-id|last> --type T")
	}
	fs := newFlags("chat save", env)
	wt := fs.String("type", "", "work type: analyze, summarize, document or report")
	if _, err := parse(fs, args[3:]); err != nil {
		return err
	}
	msgID := args[2]
	if msgID == "last" {
		s, err := svc.GetChat(args[1])
		if err != nil {
			return err
		}
		msgID = lastReplyID(s)
	}
	out, err := svc.SaveChatReply(ctx, args[1], msgID, *wt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "saved %q as output %s in notebook %s\n", out.Title, out.OutputID, out.NotebookID)
	return wrapWrite(err)
}

func lastReplyID(s chat.Session) string {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role == chat.RoleAssistant {
			return s.Messages[i].ID
		}
	}
	return "none"
}
