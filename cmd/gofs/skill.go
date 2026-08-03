package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	kitup "github.com/lathe-cli/kitup/go"
	"github.com/samzong/gofs/internal/gofsskill"
)

func runSkill(args []string, stdin io.Reader, stdout, stderr io.Writer, stdinTTY bool) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, "Usage: gofs skill install [flags]")
		return flag.ErrHelp
	}
	if args[0] != "install" {
		return fmt.Errorf("unknown skill command %q", args[0])
	}

	fs := flag.NewFlagSet("gofs skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scope := fs.String("scope", "", kitup.InstallUX.ScopeFlag)
	var agents stringSlice
	fs.Var(&agents, "agent", kitup.InstallUX.AgentFlag)
	yes := fs.Bool("yes", false, kitup.InstallUX.YesFlag)
	fs.BoolVar(yes, "y", false, kitup.InstallUX.YesFlag)
	dryRun := fs.Bool("dry-run", false, kitup.InstallUX.DryRunFlag)
	force := fs.Bool("force", false, kitup.InstallUX.ForceFlag)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("gofs skill install does not accept positional arguments")
	}

	scopeSet := false
	fs.Visit(func(value *flag.Flag) {
		if value.Name == "scope" {
			scopeSet = true
		}
	})
	parsed := kitup.ParseInstallFlags(kitup.InstallFlagValues{
		Scope:    *scope,
		ScopeSet: scopeSet,
		Agents:   agents,
		Yes:      *yes,
		DryRun:   *dryRun,
		Force:    *force,
	})
	if err := kitup.InstallFlagError(parsed.Errors); err != nil {
		return err
	}

	report, err := kitup.RunBundledSkillInstall(kitup.InstallWorkflowOptions{
		InstallOptions: kitup.InstallOptions{
			AppID:       "gofs",
			SkillBundle: kitup.FSBundle(gofsskill.FS, gofsskill.Root),
			Scope:       parsed.Scope,
			Agents:      parsed.Agents,
			Force:       parsed.Force,
		},
		Yes:          parsed.Yes,
		DryRun:       parsed.DryRun,
		StdinTTY:     stdinTTY,
		DefaultScope: kitup.UserScope,
		ScopeSet:     parsed.ScopeSet,
		PromptScope:  true,
		In:           stdin,
		Out:          stdout,
		Err:          stderr,
	})
	if err != nil {
		return err
	}
	return kitup.InstallWorkflowError(report)
}

func stdinTTY(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
