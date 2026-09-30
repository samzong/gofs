package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	kitup "github.com/lathe-cli/kitup/go"
	"github.com/samzong/gofs/internal/gofsskill"
)

const (
	skillAppID = "gofs"
	skillName  = "gofs"
)

func runSkill(args []string, stdin io.Reader, stdout, stderr io.Writer, stdinTTY bool) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, "Usage: gofs skill <install|status|uninstall> [flags]")
		return flag.ErrHelp
	}
	switch args[0] {
	case "install":
		return runSkillInstall(args[1:], stdin, stdout, stderr, stdinTTY)
	case "status":
		return runSkillStatus(args[1:], stdout, stderr)
	case "uninstall":
		return runSkillUninstall(args[1:], stdin, stdout, stderr, stdinTTY)
	default:
		return fmt.Errorf("unknown skill command %q", args[0])
	}
}

func runSkillInstall(args []string, stdin io.Reader, stdout, stderr io.Writer, stdinTTY bool) error {
	fs := flag.NewFlagSet("gofs skill install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scope := fs.String("scope", "", kitup.InstallUX.ScopeFlag)
	var agents stringSlice
	fs.Var(&agents, "agent", kitup.InstallUX.AgentFlag)
	yes := fs.Bool("yes", false, kitup.InstallUX.YesFlag)
	fs.BoolVar(yes, "y", false, kitup.InstallUX.YesFlag)
	dryRun := fs.Bool("dry-run", false, kitup.InstallUX.DryRunFlag)
	force := fs.Bool("force", false, kitup.InstallUX.ForceFlag)
	if err := parseSkillFlags(fs, args, "gofs skill install does not accept positional arguments"); err != nil {
		return err
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
			AppID:       skillAppID,
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

func runSkillStatus(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("gofs skill status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scope := fs.String("scope", string(kitup.UserScope), kitup.InstallUX.ScopeFlag)
	var agents stringSlice
	fs.Var(&agents, "agent", kitup.InstallUX.AgentFlag)
	jsonOutput := fs.Bool("json", false, "Write a structured JSON report")
	if err := parseSkillFlags(fs, args, "gofs skill status does not accept positional arguments"); err != nil {
		return err
	}

	parsed := kitup.ParseInstallFlags(kitup.InstallFlagValues{
		Scope:    *scope,
		ScopeSet: true,
		Agents:   agents,
	})
	if err := kitup.InstallFlagError(parsed.Errors); err != nil {
		return err
	}

	report, err := kitup.StatusBundledSkill(kitup.StatusOptions{
		AppID:     skillAppID,
		SkillName: skillName,
		Scope:     parsed.Scope,
		Agents:    parsed.Agents,
	})
	if err != nil {
		return err
	}
	if *jsonOutput {
		if err := writeJSON(stdout, report); err != nil {
			return err
		}
	} else {
		renderStatusReport(stdout, report)
	}
	return statusReportError(report)
}

func runSkillUninstall(args []string, stdin io.Reader, stdout, stderr io.Writer, stdinTTY bool) error {
	fs := flag.NewFlagSet("gofs skill uninstall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scope := fs.String("scope", string(kitup.UserScope), kitup.InstallUX.ScopeFlag)
	var agents stringSlice
	fs.Var(&agents, "agent", kitup.InstallUX.AgentFlag)
	yes := fs.Bool("yes", false, "Skip uninstall confirmation")
	fs.BoolVar(yes, "y", false, "Skip uninstall confirmation")
	jsonOutput := fs.Bool("json", false, "Write a structured JSON report")
	if err := parseSkillFlags(fs, args, "gofs skill uninstall does not accept positional arguments"); err != nil {
		return err
	}

	parsed := kitup.ParseInstallFlags(kitup.InstallFlagValues{
		Scope:    *scope,
		ScopeSet: true,
		Agents:   agents,
		Yes:      *yes,
	})
	if err := kitup.InstallFlagError(parsed.Errors); err != nil {
		return err
	}
	if !parsed.Yes && !stdinTTY {
		return errors.New("kitup: uninstall requires --yes when stdin is not a TTY")
	}

	status, err := kitup.StatusBundledSkill(kitup.StatusOptions{
		AppID:     skillAppID,
		SkillName: skillName,
		Scope:     parsed.Scope,
		Agents:    parsed.Agents,
	})
	if err != nil {
		return err
	}
	if len(status.Conflicts)+len(status.Errors) > 0 {
		report := uninstallReportFromStatus(status)
		if err := writeUninstallOutput(stdout, report, *jsonOutput); err != nil {
			return err
		}
		return errors.New("kitup: uninstall has conflicts")
	}
	if len(status.Installed) == 0 {
		return writeUninstallOutput(stdout, uninstallReportFromStatus(status), *jsonOutput)
	}

	promptOut := stdout
	if *jsonOutput {
		promptOut = stderr
	}
	if !*jsonOutput {
		renderStatusReport(promptOut, status)
	}
	if !parsed.Yes {
		confirmed, err := confirmUninstall(stdin, promptOut, len(status.Installed))
		if err != nil {
			return err
		}
		if !confirmed {
			_, _ = fmt.Fprintln(promptOut, "Uninstall canceled.")
			if *jsonOutput {
				return writeJSON(stdout, uninstallReportFromStatus(status))
			}
			return nil
		}
	}

	report, err := kitup.UninstallBundledSkill(kitup.UninstallOptions{
		AppID:     skillAppID,
		SkillName: skillName,
		Scope:     parsed.Scope,
		Agents:    parsed.Agents,
	})
	if err != nil {
		return err
	}
	if err := writeUninstallOutput(stdout, report, *jsonOutput); err != nil {
		return err
	}
	if len(report.Conflicts)+len(report.Errors) > 0 {
		return errors.New("kitup: uninstall failed")
	}
	return nil
}

func parseSkillFlags(fs *flag.FlagSet, args []string, extraArgsErr string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(extraArgsErr)
	}
	return nil
}

func statusReportError(report kitup.StatusReport) error {
	if len(report.Conflicts) > 0 {
		return errors.New("kitup: status has conflicts")
	}
	if len(report.Errors) > 0 {
		return errors.New("kitup: status failed")
	}
	return nil
}

func uninstallReportFromStatus(status kitup.StatusReport) kitup.UninstallReport {
	report := kitup.UninstallReport{
		Removed:   []kitup.TargetResult{},
		Skipped:   []kitup.TargetStatus{},
		Conflicts: append([]kitup.TargetStatus{}, status.Conflicts...),
		Errors:    append([]kitup.ReportError{}, status.Errors...),
	}
	for _, target := range status.Missing {
		report.Skipped = append(report.Skipped, kitup.TargetStatus{TargetResult: target, Reason: "missing"})
	}
	return report
}

func confirmUninstall(in io.Reader, out io.Writer, count int) (bool, error) {
	if _, err := fmt.Fprintf(out, "Remove %d installed target(s)? [y/N] ", count); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func writeUninstallOutput(out io.Writer, report kitup.UninstallReport, jsonOutput bool) error {
	if jsonOutput {
		return writeJSON(out, report)
	}
	renderUninstallReport(out, report)
	return nil
}

func writeJSON(out io.Writer, value any) error {
	return json.NewEncoder(out).Encode(value)
}

func renderStatusReport(out io.Writer, report kitup.StatusReport) {
	for _, item := range report.Installed {
		_, _ = fmt.Fprintf(out, "installed\t%s\t%s\n", targetHosts(item.TargetResult), item.TargetDir)
	}
	for _, item := range report.Missing {
		_, _ = fmt.Fprintf(out, "missing\t%s\t%s\n", targetHosts(item), item.TargetDir)
	}
	for _, item := range report.Conflicts {
		_, _ = fmt.Fprintf(out, "conflict\t%s\t%s\t%s\n", targetHosts(item.TargetResult), item.TargetDir, item.Reason)
	}
	for _, item := range report.Errors {
		_, _ = fmt.Fprintf(out, "error\t%s\n", item.Reason)
	}
}

func renderUninstallReport(out io.Writer, report kitup.UninstallReport) {
	for _, item := range report.Removed {
		_, _ = fmt.Fprintf(out, "removed\t%s\t%s\n", targetHosts(item), item.TargetDir)
	}
	for _, item := range report.Skipped {
		_, _ = fmt.Fprintf(out, "skipped\t%s\t%s\t%s\n", targetHosts(item.TargetResult), item.TargetDir, item.Reason)
	}
	for _, item := range report.Conflicts {
		_, _ = fmt.Fprintf(out, "conflict\t%s\t%s\t%s\n", targetHosts(item.TargetResult), item.TargetDir, item.Reason)
	}
	for _, item := range report.Errors {
		_, _ = fmt.Fprintf(out, "error\t%s\n", item.Reason)
	}
}

func targetHosts(target kitup.TargetResult) string {
	if target.HostID != "" {
		return target.HostID
	}
	return strings.Join(target.HostIDs, ",")
}

func stdinTTY(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
