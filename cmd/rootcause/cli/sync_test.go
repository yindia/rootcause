package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rootcause/pkg/server"
)

// TestUnifiedSyncListAgentsIncludesBothSurfaces verifies the command lists
// agents and shows columns for both commands (prompts) and skills directories.
func TestUnifiedSyncListAgentsIncludesBothSurfaces(t *testing.T) {
	run := func(context.Context, server.Options) error { return nil }
	var out bytes.Buffer
	if err := Execute(context.Background(), []string{"sync", "--list-agents"}, run, "test", &out); err != nil {
		t.Fatalf("execute: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "AGENT") || !strings.Contains(text, "COMMANDS") || !strings.Contains(text, "SKILLS") {
		t.Errorf("expected header row with COMMANDS and SKILLS, got:\n%s", text)
	}
	if !strings.Contains(text, "claude") {
		t.Errorf("expected claude listed, got:\n%s", text)
	}
}

// TestUnifiedSyncOverwriteDefaultIsSafe ensures the second invocation of sync
// with the same input does not clobber existing files (i.e. --overwrite
// defaults to false).
func TestUnifiedSyncOverwriteDefaultIsSafe(t *testing.T) {
	tmp := t.TempDir()
	// Drop a custom prompt the sync should pick up automatically.
	promptsDir := filepath.Join(tmp, ".rootcause", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("mkdir prompts: %v", err)
	}
	src := `---
name: t_sample
description: Test prompt
arguments:
  - name: w
    required: true
---

Body $1.
`
	if err := os.WriteFile(filepath.Join(promptsDir, "t-sample.md"), []byte(src), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	t.Setenv("ROOTCAUSE_PROMPTS_DIR", promptsDir)

	run := func(context.Context, server.Options) error { return nil }
	var out bytes.Buffer
	if err := Execute(context.Background(), []string{
		"sync", "--agent", "claude", "--project-dir", tmp, "--prompts-only",
	}, run, "test", &out); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	dest := filepath.Join(tmp, ".claude", "commands", "t-sample.md")
	stat, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("expected file to be written, got %v", err)
	}
	mtime := stat.ModTime()

	// Mutate the generated file so we can detect overwrite.
	if err := os.WriteFile(dest, []byte("EDITED BY USER\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	// Second sync — without --overwrite, should skip the existing file.
	out.Reset()
	if err := Execute(context.Background(), []string{
		"sync", "--agent", "claude", "--project-dir", tmp, "--prompts-only",
	}, run, "test", &out); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	body, _ := os.ReadFile(dest)
	if string(body) != "EDITED BY USER\n" {
		t.Errorf("expected hand-edited file to survive second sync without --overwrite, got:\n%s", body)
	}
	if stat2, _ := os.Stat(dest); stat2.ModTime().Before(mtime) {
		t.Errorf("file mtime regressed — sync wrote over user edit")
	}

	// Now with --overwrite explicitly, the file should be replaced.
	out.Reset()
	if err := Execute(context.Background(), []string{
		"sync", "--agent", "claude", "--project-dir", tmp, "--prompts-only", "--overwrite",
	}, run, "test", &out); err != nil {
		t.Fatalf("overwrite sync: %v", err)
	}
	body, _ = os.ReadFile(dest)
	if strings.Contains(string(body), "EDITED BY USER") {
		t.Errorf("expected --overwrite to replace user edit, but it survived")
	}
}

// TestUnifiedSyncRejectsBothPromptsOnlyAndSkillsOnly enforces the mutual
// exclusivity guardrail.
func TestUnifiedSyncRejectsBothPromptsOnlyAndSkillsOnly(t *testing.T) {
	run := func(context.Context, server.Options) error { return nil }
	var out bytes.Buffer
	err := Execute(context.Background(), []string{
		"sync", "--agent", "claude", "--prompts-only", "--skills-only",
	}, run, "test", &out)
	if err == nil {
		t.Fatalf("expected error when both --prompts-only and --skills-only are set")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' error, got: %v", err)
	}
}

// TestResolveSyncConfigPathChain verifies the same resolution order the
// server uses: explicit --config, then ROOTCAUSE_CONFIG env, then standard
// filesystem candidates. Without one of these, sync falls back to env vars
// and default prompt-search behavior.
func TestResolveSyncConfigPathChain(t *testing.T) {
	t.Setenv("ROOTCAUSE_CONFIG", "")

	// 1. Explicit flag wins.
	if got := resolveSyncConfigPath("/tmp/x.yaml"); got != "/tmp/x.yaml" {
		t.Errorf("explicit flag should win, got %q", got)
	}

	// 2. ROOTCAUSE_CONFIG when no flag.
	t.Setenv("ROOTCAUSE_CONFIG", "/tmp/env.yaml")
	if got := resolveSyncConfigPath(""); got != "/tmp/env.yaml" {
		t.Errorf("env should win when no flag, got %q", got)
	}

	// 3. Falls back to "" (no candidates exist) when neither set.
	t.Setenv("ROOTCAUSE_CONFIG", "")
	t.Setenv("HOME", t.TempDir()) // ensures no ~/.rootcause/config.yaml exists
	if got := resolveSyncConfigPath(""); got != "" {
		// Could match ./config.yaml in working dir — just assert it doesn't
		// pick a phantom path.
		if _, statErr := os.Stat(got); statErr != nil {
			t.Errorf("resolveSyncConfigPath returned non-existent path %q", got)
		}
	}
}

// TestUnifiedSyncCustomPromptIsIncludedByDefault confirms a user's custom
// prompt in ~/.rootcause/prompts/ is picked up without --include-custom (the
// flag is gone; customs are on by default).
func TestUnifiedSyncCustomPromptIsIncludedByDefault(t *testing.T) {
	tmp := t.TempDir()
	promptsDir := filepath.Join(tmp, ".rootcause", "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "custom-only.md"), []byte(`---
name: custom_only
description: not a builtin
---

Hello.
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("ROOTCAUSE_PROMPTS_DIR", promptsDir)

	run := func(context.Context, server.Options) error { return nil }
	var out bytes.Buffer
	if err := Execute(context.Background(), []string{
		"sync", "--list", "--prompts-only",
	}, run, "test", &out); err != nil {
		t.Fatalf("sync --list: %v", err)
	}
	if !strings.Contains(out.String(), "custom_only") {
		t.Errorf("expected custom prompt to appear in --list output by default, got:\n%s", out.String())
	}
}
