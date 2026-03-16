package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"rootcause/internal/skills/catalog"
	"rootcause/pkg/server"
)

func TestTargetFilePathFormats(t *testing.T) {
	root := "/tmp/skills"
	if got := targetFilePath(root, "k8s-incident", formatSkillMD); got != filepath.Join(root, "k8s-incident", "SKILL.md") {
		t.Fatalf("unexpected SKILL.md path: %s", got)
	}
	if got := targetFilePath(root, "k8s-incident", formatMDC); got != filepath.Join(root, "k8s-incident.mdc") {
		t.Fatalf("unexpected .mdc path: %s", got)
	}
	if got := targetFilePath(root, "k8s-incident", formatMD); got != filepath.Join(root, "k8s-incident.md") {
		t.Fatalf("unexpected .md path: %s", got)
	}
}

func TestSyncSkillsForTargetCopiesFiles(t *testing.T) {
	projectDir := t.TempDir()
	source := filepath.Join(projectDir, "skills", "claude")
	if err := os.MkdirAll(filepath.Join(source, "k8s-incident"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	content := []byte("# k8s-incident\n")
	if err := os.WriteFile(filepath.Join(source, "k8s-incident", "SKILL.md"), content, 0o644); err != nil {
		t.Fatalf("write source skill: %v", err)
	}
	skills := []catalog.Skill{{Name: "k8s-incident", Path: "skills/claude/k8s-incident/SKILL.md"}}

	count, dest, err := syncSkillsForTarget(source, projectDir, agentTargets["claude"], skills, true, false)
	if err != nil {
		t.Fatalf("syncSkillsForTarget: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 copied file, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(dest, "k8s-incident", "SKILL.md")); err != nil {
		t.Fatalf("expected copied SKILL.md: %v", err)
	}

	count, dest, err = syncSkillsForTarget(source, projectDir, agentTargets["cursor"], skills, true, false)
	if err != nil {
		t.Fatalf("sync cursor: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 copied file for cursor, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(dest, "k8s-incident.mdc")); err != nil {
		t.Fatalf("expected copied .mdc: %v", err)
	}

	count, dest, err = syncSkillsForTarget(source, projectDir, agentTargets["copilot"], skills, true, false)
	if err != nil {
		t.Fatalf("sync copilot: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 copied file for copilot, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(dest, "k8s-incident.md")); err != nil {
		t.Fatalf("expected copied .md: %v", err)
	}
}

func TestSyncSkillsForTargetNoOverwrite(t *testing.T) {
	projectDir := t.TempDir()
	source := filepath.Join(projectDir, "skills", "claude")
	if err := os.MkdirAll(filepath.Join(source, "k8s-incident"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "k8s-incident", "SKILL.md"), []byte("new"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	skills := []catalog.Skill{{Name: "k8s-incident", Path: "skills/claude/k8s-incident/SKILL.md"}}
	dest := filepath.Join(projectDir, ".claude", "skills", "k8s-incident")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	destFile := filepath.Join(dest, "SKILL.md")
	if err := os.WriteFile(destFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("write existing dest: %v", err)
	}

	count, _, err := syncSkillsForTarget(source, projectDir, agentTargets["claude"], skills, false, false)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 copied when overwrite disabled, got %d", count)
	}
	data, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(data) != "old" {
		t.Fatalf("expected existing file untouched, got %q", string(data))
	}
}

func TestSelectedSkillsFromFilter(t *testing.T) {
	m, err := catalog.Load()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	skills, err := selectedSkills(m, []string{"k8s-incident", "k8s-helm"})
	if err != nil {
		t.Fatalf("selectedSkills: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestExecuteListAgentsWithoutRunServer(t *testing.T) {
	called := false
	run := func(context.Context, server.Options) error {
		called = true
		return nil
	}
	var out bytes.Buffer
	err := Execute(context.Background(), []string{"sync-skills", "--list-agents"}, run, "test", &out)
	if err != nil {
		t.Fatalf("execute list-agents: %v", err)
	}
	if called {
		t.Fatalf("expected runServer not to be called for sync-skills")
	}
	if out.Len() == 0 {
		t.Fatalf("expected list output")
	}
}
