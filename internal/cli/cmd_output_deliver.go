package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/samautomateai/tessivo/internal/service"
)

// outputIssues prints the tracker-ready issues an Analyze output's findings make.
func outputIssues(_ context.Context, svc *service.Service, a outputArgs, env Env) error {
	issues, err := svc.Issues(a.nb, a.out)
	if err != nil {
		return err
	}
	if a.json {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return wrapWrite(enc.Encode(issues))
	}
	for _, is := range issues {
		_, err := fmt.Fprintf(env.Stdout, "%d. [%s] %s\n   labels: %s\n", is.Index+1, is.Severity, is.Title,
			strings.Join(is.Labels, ", "))
		if err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}

// outputPush files an Analyze output's findings as issues: --to NAME [--only 1,3] [--again].
func outputPush(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	if a.to == "" {
		return usagef("output push: want --to <integration>")
	}
	var picked []int
	for _, f := range strings.FieldsFunc(a.only, func(r rune) bool { return r == ',' || r == ' ' }) {
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 {
			return usagef("output push --only: want finding numbers such as 1,3")
		}
		picked = append(picked, n-1)
	}
	res, err := svc.PushIssues(ctx, a.nb, a.out, a.to, picked, a.again)
	for _, d := range res.Sent {
		if _, werr := fmt.Fprintf(env.Stdout, "%d. %s\n", d.Finding+1, d.URL); werr != nil {
			return wrapWrite(werr)
		}
	}
	if len(res.Skipped) > 0 {
		_, _ = fmt.Fprintf(env.Stderr, "skipped %d finding(s) already sent to %s (use --again to send them again)\n",
			len(res.Skipped), a.to)
	}
	return err
}

// outputPublish sends an output as a page or file: --to NAME.
func outputPublish(ctx context.Context, svc *service.Service, a outputArgs, env Env) error {
	if a.to == "" {
		return usagef("output publish: want --to <integration>")
	}
	d, err := svc.Publish(ctx, a.nb, a.out, a.to)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(env.Stdout, d.URL)
	return wrapWrite(err)
}

func integrationTest(ctx context.Context, svc *service.Service, name string, env Env) error {
	if err := svc.TestIntegration(ctx, name); err != nil {
		return err
	}
	_, err := fmt.Fprintf(env.Stdout, "%s: ok\n", name)
	return wrapWrite(err)
}
