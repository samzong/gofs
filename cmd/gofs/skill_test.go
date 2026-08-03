package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSkillInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
		t.Fatalf("runSkill() error = %v, stderr = %q", err, stderr.String())
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
