package cli

import (
	"context"
	"fmt"

	"github.com/samautomateai/tessivo/internal/service"
)

const profileHelp = `tess profile list                 settings profiles; * marks the active one
tess profile save <name>          save the current providers, aliases, defaults and limits as a profile
tess profile use <name>           switch (the active profile keeps your edits first)
tess profile delete <name>        remove a profile (the settings stay)`

// cmdProfile manages settings profiles (SET-10).
func cmdProfile(_ context.Context, args []string, env Env) error {
	if len(args) == 0 || (args[0] != subList && len(args) != 2) || (args[0] == subList && len(args) != 1) {
		return usagef("profile: want a subcommand\n%s", profileHelp)
	}
	f, err := loadConfig(env)
	if err != nil {
		return err
	}
	svc, err := newService(f, env)
	if err != nil {
		return err
	}
	info, err := profileRun(svc, args)
	if err != nil {
		return err
	}
	return profilePrint(info, env)
}

func profileRun(svc *service.Service, args []string) (service.ProfilesInfo, error) {
	switch args[0] {
	case subList:
		return svc.Profiles(), nil
	case "save":
		return svc.SaveProfile(args[1])
	case "use":
		return svc.UseProfile(args[1])
	case "delete":
		return svc.DeleteProfile(args[1])
	}
	return service.ProfilesInfo{}, usagef("profile: unknown subcommand %q\n%s", args[0], profileHelp)
}

func profilePrint(info service.ProfilesInfo, env Env) error {
	for _, p := range info.Profiles {
		mark := " "
		if p.Name == info.Active {
			mark = "*"
		}
		if _, err := fmt.Fprintf(env.Stdout, "%s %s  %d providers  %s\n", mark, p.Name, p.Providers,
			p.DefaultModel); err != nil {
			return wrapWrite(err)
		}
	}
	return nil
}
