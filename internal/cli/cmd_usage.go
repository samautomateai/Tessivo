package cli

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/config"
)

// cmdUsage prints today's token use and the limits.
func cmdUsage(_ []string, env Env) error {
	f, err := config.Load(env.ConfigPath)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	u, err := svc.Usage()
	if err != nil {
		return err
	}
	limit := func(n int) string {
		if n <= 0 {
			return "none"
		}
		return fmt.Sprint(n)
	}
	pct := ""
	if u.DayLimit > 0 {
		pct = fmt.Sprintf(" (%d%%)", u.Tokens*100/u.DayLimit)
	}
	_, err = fmt.Fprintf(env.Stdout, "%s: %d tokens used%s\nday limit: %s · run limit: %s\n",
		u.Day, u.Tokens, pct, limit(u.DayLimit), limit(u.RunLimit))
	return wrapWrite(err)
}
