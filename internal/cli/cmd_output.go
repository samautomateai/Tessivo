package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/notebook"
	"github.com/samautomateai/tessivo/internal/service"
)

const outputHelp = `tess output ask <notebook> <output-id> <question>      ask about an output (its thread)
tess output suggest <notebook> <output-id>               ask for improvements from the thread
tess output suggestions <notebook> <output-id>           list suggestions
tess output apply|dismiss <notebook> <output-id> <suggestion-id>
tess output refine <notebook> <output-id> [--from MESSAGE_ID]   save a revised version
tess output export <notebook> <output-id> [--format md|docx|pdf] [--out FILE]   save as Markdown, Word or PDF
tess output rerun <notebook> <output-id> [--model M]   run it again, optionally on another model
tess output diff <notebook> <output-id> [--against ID]   lines changed since the version it revised
tess output data <notebook> <output-id>                  print the structured result as JSON
tess output edit <notebook> <output-id> FILE|-           save edited JSON as a new version (no model call)
tess output issues <notebook> <output-id> [--json]       the tracker-ready issues an Analyze output's findings make`

// outputCommands maps each subcommand to its handler and the positional
// arguments it needs after the notebook and output (-1: one or more).
var outputCommands = map[string]struct {
	run  func(context.Context, *service.Service, outputArgs, Env) error
	args int
}{
	"ask": {outputAsk, -1}, "suggest": {outputSuggest, 0}, "suggestions": {outputSuggestions, 0},
	"apply": {outputApply, 1}, "dismiss": {outputDismiss, 1}, "refine": {outputRefine, 0},
	subExport: {outputExport, 0},
	"rerun":   {outputRerun, 0},
	"diff":    {outputDiff, 0},
	"data":    {outputData, 0},
	"edit":    {outputEdit, 1},
	"issues":  {outputIssues, 0},
}

type outputArgs struct {
	nb, out string   // notebook folder (absolute) and output ID
	rest    []string // the remaining positional arguments
	from    string   // refine --from
	format  string   // export --format
	file    string   // export --out
	model   string   // rerun --model
	against string   // diff --against
	json    bool     // issues --json
}

func cmdOutput(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("output: want a subcommand\n%s", outputHelp)
	}
	c, ok := outputCommands[args[0]]
	if !ok {
		return usagef("output: unknown subcommand %q\n%s", args[0], outputHelp)
	}
	fs := newFlags("output "+args[0], env)
	fs.Usage = func() { _, _ = fmt.Fprintln(env.Stderr, outputHelp) }
	from := fs.String("from", "", "refine: revise from this thread reply (default: the latest)")
	format := fs.String("format", service.FormatMarkdown, "export: md, docx or pdf")
	model := fs.String("model", "", "rerun: the model to use (default: the settings that apply now)")
	against := fs.String("against", "", "diff: the version to compare with (default: the one it revised)")
	out := fs.String("out", "", "export: file to write (default: the output's title in this folder)")
	jsonOut := fs.Bool("json", false, "issues: print JSON")
	pos, err := parse(fs, args[1:])
	if err != nil {
		return err
	}
	if len(pos) < 2 || (c.args >= 0 && len(pos) != 2+c.args) || (c.args < 0 && len(pos) < 3) {
		return usagef("output %s: wrong arguments\n%s", args[0], outputHelp)
	}
	dir, err := filepath.Abs(pos[0])
	if err != nil {
		return fmt.Errorf("notebook %s: %w", pos[0], err)
	}
	if _, err := notebook.Open(dir); err != nil {
		return err
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	svc.NotebookDirs = []string{dir}
	a := outputArgs{
		nb: dir, out: pos[1], rest: pos[2:], from: *from, format: *format, file: *out, model: *model, against: *against,
		json: *jsonOut,
	}
	return cliErr(c.run(ctx, svc, a, env))
}

func outputAsk(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	in := service.ChatSend{Text: strings.Join(a.rest, " ")}
	streamed := false
	reply, err := svc.SendThread(ctx, a.nb, a.out, in, func(d string) {
		streamed = true
		_, _ = fmt.Fprint(env.Stdout, d)
	}, nil)
	if !streamed {
		_, _ = fmt.Fprint(env.Stdout, reply.Assistant.Content) // providers that do not stream
	}
	_, _ = fmt.Fprintln(env.Stdout)
	if err != nil {
		return err
	}
	return printSuggestions(env, reply.Suggestions)
}

func printSuggestions(env Env, list []notebook.Suggestion) error {
	for _, sg := range list {
		line := fmt.Sprintf("suggestion %s %s: %s", sg.ID, sg.Kind, oneLine(sg.Text, 200))
		if _, err := fmt.Fprintln(env.Stdout, line); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}

func outputSuggest(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	list, err := svc.GenerateSuggestions(ctx, a.nb, a.out)
	if err != nil {
		return err
	}
	plain := make([]notebook.Suggestion, 0, len(list))
	for _, info := range list {
		plain = append(plain, info.Suggestion)
	}
	if len(plain) == 0 {
		_, err := fmt.Fprintln(env.Stdout, "no suggestions")
		return wrapWrite(err)
	}
	return printSuggestions(env, plain)
}

func outputSuggestions(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	list, err := svc.ListSuggestions(a.nb, a.out)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tKIND\tSTATUS\tTARGET\tTEXT")
	for _, sg := range list {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", sg.ID, sg.Kind, sg.Status, targetLabel(sg.Target),
			oneLine(sg.Text, 60))
	}
	return wrapWrite(tw.Flush())
}

// targetLabel says what applying a prompt suggestion changes.
func targetLabel(tg *service.PromptTarget) string {
	switch {
	case tg == nil:
		return "-"
	case tg.PromptName != "":
		return fmt.Sprintf("prompt %s (%d schedules)", tg.PromptName, len(tg.Schedules))
	case tg.ScheduleID != "":
		return "schedule " + tg.ScheduleID
	default:
		return "new prompt"
	}
}

func outputApply(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	res, err := svc.ApplySuggestion(a.nb, a.out, a.rest[0])
	if err != nil {
		return err
	}
	msg := "applied " + res.Suggestion.ID
	if res.PromptName != "" {
		msg += "; saved as prompt " + res.PromptName
	}
	_, err = fmt.Fprintln(env.Stdout, msg)
	return wrapWrite(err)
}

func outputDismiss(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	sg, err := svc.DismissSuggestion(a.nb, a.out, a.rest[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, "dismissed "+sg.ID)
	return wrapWrite(err)
}

func outputRefine(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	res, err := svc.NewVersion(ctx, a.nb, a.out, a.from)
	if err != nil {
		return err
	}
	nb, err := notebook.Open(a.nb)
	if err != nil {
		return err
	}
	outs, err := nb.ListOutputs()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "saved v%d %s\n", service.Versions(outs)[res.OutputID], res.OutputID)
	return wrapWrite(err)
}

// oneLine flattens s and cuts it to n runes.
func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// outputExport writes an output as Markdown or Word.
func outputExport(_ context.Context, _ *service.Service, a outputArgs, env Env) error {
	nb, err := notebook.Open(a.nb)
	if err != nil {
		return err
	}
	o, md, err := nb.GetOutput(a.out)
	if err != nil {
		return err
	}
	f, err := service.RenderExport(o.Title, md, a.format)
	if err != nil {
		return err
	}
	path := a.file
	if path == "" {
		path = f.Name
	}
	if err := os.WriteFile(path, f.Data, 0o600); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	_, err = fmt.Fprintf(env.Stdout, "wrote %s\n", path)
	return wrapWrite(err)
}

// outputRerun runs the run behind an output again (optionally on --model)
// and prints the new output.
func outputRerun(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	req, err := svc.RerunRequest(a.nb, a.out, a.model)
	if err != nil {
		return err
	}
	out, err := svc.Run(ctx, req, nil)
	if err != nil {
		return err
	}
	if err := writeResult(out, runOpts{}, env); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stderr, "saved output %s (model %s)\n", out.OutputID, out.Result.Model)
	return wrapWrite(err)
}

// outputDiff prints the lines a version changed against its parent (or --against).
func outputDiff(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	d, err := svc.DiffOutputs(a.nb, a.out, a.against)
	if err != nil {
		return err
	}
	for _, l := range d.Lines {
		if _, err := fmt.Fprintf(env.Stdout, "%c %s\n", l.Op, l.Text); err != nil {
			return wrapWrite(err)
		}
	}
	_, err = fmt.Fprintf(env.Stderr, "%s → %s: %d added, %d removed\n", d.From, d.To, d.Counts.Added, d.Counts.Removed)
	return wrapWrite(err)
}
