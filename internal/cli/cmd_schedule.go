package cli

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/samautomateai/tessivo/internal/cron"
	"github.com/samautomateai/tessivo/internal/notebook"
)

const subEnable = "enable"

func cmdSchedule(args []string, env Env) error {
	if len(args) < 2 {
		return usagef("schedule: want add|list|remove|enable|disable <notebook> ... or next <cron>")
	}
	sub := args[0]
	if sub == "next" {
		return scheduleNext(args[1:], env)
	}
	nb, err := notebook.Open(args[1])
	if err != nil {
		return err
	}
	rest := args[2:]
	switch sub {
	case subAdd:
		return scheduleAdd(nb, rest, env)
	case subList:
		return scheduleList(nb, env)
	case subRemove, subEnable, "disable":
		if len(rest) != 1 {
			return usagef("schedule %s: want <notebook> <schedule-id>", sub)
		}
		return scheduleEdit(nb, sub, rest[0], env)
	default:
		return usagef("schedule: unknown subcommand %q", sub)
	}
}

func scheduleAdd(nb *notebook.Notebook, args []string, env Env) error {
	fs := newFlags("schedule add", env)
	spec := fs.String("cron", "", `cron expression, e.g. "0 8 * * MON" or "@daily"`)
	prompt := fs.String("prompt", "", "prompt text (default: the work type's starter prompt)")
	promptName := fs.String("prompt-name", "", "use a saved notebook prompt")
	missed := fs.String("missed", notebook.MissedSkip, "after downtime: skip or run_once")
	var vars multiFlag
	fs.Var(&vars, "var", "prompt variable name=value (repeatable)")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	vm, err := parseVars(vars)
	if err != nil {
		return err
	}
	sch := notebook.Schedule{
		ID: notebook.NewID(), Cron: *spec, Prompt: *prompt, PromptName: *promptName, Vars: vm, Missed: *missed,
		Enabled: true, CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := sch.Validate(); err != nil {
		return usageError{err.Error()}
	}
	list, err := nb.Schedules()
	if err != nil {
		return err
	}
	if err := nb.SaveSchedules(append(list, sch)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(env.Stdout, "added schedule %s (%s); runs while 'tess serve' or the desktop app is running\n",
		sch.ID, sch.Cron)
	return wrapWrite(err)
}

func scheduleList(nb *notebook.Notebook, env Env) error {
	list, err := nb.Schedules()
	if err != nil {
		return err
	}
	st, err := nb.LoadScheduleState()
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tCRON\tON\tMISSED\tNEXT\tLAST")
	now := time.Now()
	for _, s := range list {
		next, last := "-", "-"
		if c, err := cron.Parse(s.Cron, time.Local); err == nil && s.Enabled {
			if n, err := c.Next(now); err == nil {
				next = n.Format("2006-01-02 15:04 MST")
			}
		}
		if rs := st.Schedules[s.ID]; rs != nil && len(rs.History) > 0 {
			last = rs.History[0].StartedAt + " " + rs.History[0].Status
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%v\t%s\t%s\t%s\n", s.ID, s.Cron, s.Enabled, s.Missed, next, last)
	}
	return wrapWrite(tw.Flush())
}

func scheduleEdit(nb *notebook.Notebook, sub, id string, env Env) error {
	list, err := nb.Schedules()
	if err != nil {
		return err
	}
	for i, s := range list {
		if s.ID != id {
			continue
		}
		if sub == subRemove {
			list = append(list[:i], list[i+1:]...)
		} else {
			list[i].Enabled = sub == subEnable
		}
		if err := nb.SaveSchedules(list); err != nil {
			return err
		}
		_, err := fmt.Fprintf(env.Stdout, "%sd schedule %s\n", sub, id)
		return wrapWrite(err)
	}
	return fmt.Errorf("no schedule %q", id)
}

func scheduleNext(args []string, env Env) error {
	fs := newFlags("schedule next", env)
	count := fs.Int("count", 5, "how many upcoming times to show (max 50)")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *count < 1 || *count > 50 {
		return usagef(`schedule next: want one quoted expression, e.g. tess schedule next "0 8 * * MON"`)
	}
	c, err := cron.Parse(pos[0], time.Local)
	if err != nil {
		return usageError{err.Error()}
	}
	t := time.Now()
	for i := 0; i < *count; i++ {
		if t, err = c.Next(t); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(env.Stdout, t.Format("Mon 2006-01-02 15:04 MST")); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}
