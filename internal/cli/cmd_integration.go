package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/service"
)

const integrationHelp = `tess integration list                              trackers, wikis and folders outputs go to
tess integration add <name> --kind K [flags]        K: github, gitlab, jira, confluence, notion or folder
    --url URL --project P --token-ref SECRET --user U --issue-type T --space S --parent ID --path DIR
tess integration remove <name>
tess integration test <name>                        check the connection and the token
Tokens are never stored in the config: store one with 'tess secret set SECRET'.`

// cmdIntegration manages integrations (REP-4, REP-5).
func cmdIntegration(ctx context.Context, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("integration: want a subcommand\n%s", integrationHelp)
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	switch args[0] {
	case subList:
		return integrationList(svc, env)
	case subAdd:
		return integrationAdd(svc, args[1:], env)
	case subRemove:
		if len(args) != 2 {
			return usagef("integration remove: want <name>")
		}
		return svc.RemoveIntegration(args[1])
	case "test":
		if len(args) != 2 {
			return usagef("integration test: want <name>")
		}
		return integrationTest(ctx, svc, args[1], env)
	}
	return usagef("integration: unknown subcommand %q\n%s", args[0], integrationHelp)
}

func integrationList(svc *service.Service, env Env) error {
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	for _, in := range svc.ListIntegrations() {
		where := in.Project + in.Space + in.Parent + in.Path
		token := "token set"
		if !in.TokenSet {
			token = "no token"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", in.Name, in.Kind, in.URL(), where, token); err != nil {
			return wrapWrite(err)
		}
	}
	return wrapWrite(tw.Flush())
}

func integrationAdd(svc *service.Service, args []string, env Env) error {
	fs := newFlags("integration add", env)
	var in config.Integration
	fs.StringVar(&in.Kind, "kind", "", "github, gitlab, jira, confluence, notion or folder")
	fs.StringVar(&in.BaseURL, "url", "", "base URL (default for github and notion)")
	fs.StringVar(&in.Project, "project", "", "github owner/repo, gitlab group/name or ID, jira project key")
	fs.StringVar(&in.TokenRef, "token-ref", "", "the secret that holds the token")
	fs.StringVar(&in.User, "user", "", "jira and confluence: use Basic auth as this account (e-mail on Cloud)")
	fs.StringVar(&in.IssueType, "issue-type", "", "jira: the issue type (default Task)")
	fs.StringVar(&in.Space, "space", "", "confluence: the space key")
	fs.StringVar(&in.Parent, "parent", "", "confluence (optional) or notion: the parent page ID")
	fs.StringVar(&in.Path, "path", "", "folder: an absolute path")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usagef("integration add: want <name> and flags\n%s", integrationHelp)
	}
	in.Name = pos[0]
	info, err := svc.AddIntegration(in)
	if err != nil {
		return err
	}
	msg := "added " + info.Name
	if !info.TokenSet {
		msg += fmt.Sprintf("; store its token with: tess secret set %s", info.TokenRef)
	}
	_, err = fmt.Fprintln(env.Stdout, msg)
	return wrapWrite(err)
}
