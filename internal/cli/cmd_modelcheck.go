package cli

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"
)

// errModelsFailed makes `tess models test` exit 1 when a model does not answer.
var errModelsFailed = errors.New("models test: some models did not answer")

// modelsTest sends one tiny request to every model behind an alias or
// provider/model and prints which ones answer.
func modelsTest(ctx context.Context, args []string, env Env) error {
	fs := newFlags("models test", env)
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("models test: want <alias-or-provider/model>")
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	checks, err := svc.TestModel(ctx, pos[0])
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "MODEL\tRESULT\tTIME")
	failed := false
	for _, c := range checks {
		result := "ok"
		if !c.OK {
			result, failed = "failed: "+c.Error, true
		}
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d ms\n", c.Model, result, c.Millis)
	}
	if err := tw.Flush(); err != nil {
		return wrapWrite(err)
	}
	if failed {
		return errModelsFailed
	}
	return nil
}
