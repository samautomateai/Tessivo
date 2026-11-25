package cli

import (
	"context"
	"encoding/json"
	"fmt"
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
