package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kitup "github.com/lathe-cli/kitup/go"
)

func TestRunSkillInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := installGofsSkill(t); err != nil {
		t.Fatalf("runSkill() error = %v", err)
	}

	path := filepath.Join(home, ".agents", "skills", "gofs", "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("installed skill: %v", err)
	}
}

func TestRunSkillInstallRequiresNoninteractiveSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runSkill(
		[]string{"install", "--scope", "user"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
		false,
	)
	if err == nil {
		t.Fatal("runSkill() error = nil")
	}

	path := filepath.Join(home, ".agents", "skills", "gofs")
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("skill directory exists after rejected install: %v", statErr)
	}
}

func TestRunSkillStatusJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := installGofsSkill(t); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runSkill(
		[]string{"status", "--agent", "codex", "--json"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
		false,
	)
	if err != nil {
		t.Fatalf("runSkill() error = %v, stderr = %q", err, stderr.String())
	}

	var report kitup.StatusReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Installed) != 1 || report.Installed[0].Metadata.AppID != skillAppID {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestRunSkillUninstallRequiresYesWithoutTTY(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := installGofsSkill(t); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runSkill(
		[]string{"uninstall", "--agent", "codex"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
		false,
	)
	if err == nil {
		t.Fatal("expected non-interactive uninstall to require confirmation bypass")
	}

	path := filepath.Join(home, ".agents", "skills", "gofs", ".kitup.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestRunSkillUninstallJSONKeepsPromptOffStdout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := installGofsSkill(t); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runSkill(
		[]string{"uninstall", "--agent", "codex", "--json"},
		strings.NewReader("y\n"),
		&stdout,
		&stderr,
		true,
	)
	if err != nil {
		t.Fatalf("runSkill() error = %v, stderr = %q", err, stderr.String())
	}

	var report kitup.UninstallReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Removed) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if strings.Contains(stdout.String(), "Remove ") || !strings.Contains(stderr.String(), "Remove 1 installed target") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "gofs")); !os.IsNotExist(err) {
		t.Fatalf("expected target removed, got %v", err)
	}
}

func installGofsSkill(t *testing.T) error {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runSkill(
		[]string{"install", "--scope", "user", "--agent", "codex", "--yes"},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
		false,
	)
	if err != nil {
		t.Logf("stderr = %q", stderr.String())
	}
	return err
}
