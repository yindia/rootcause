package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rcmcp "rootcause/internal/mcp"
)

func TestSlugifyPromptName(t *testing.T) {
	cases := map[string]string{
		"observability_workload_diagnose":    "observability-workload-diagnose",
		"sre_incident_commander":   "sre-incident-commander",
		"no_underscores_here_only": "no-underscores-here-only",
		"alreadykebab":             "alreadykebab",
	}
	for in, want := range cases {
		if got := slugifyPromptName(in); got != want {
			t.Errorf("slugifyPromptName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSubstitutePositionalPreservesDefaults(t *testing.T) {
	args := []rcmcp.PromptArgument{
		{Name: "namespace", Required: true},
		{Name: "workload", Required: true},
		{Name: "duration", Required: false},
	}
	tmpl := "Investigate {{namespace}}/{{workload}} over {{duration|30m}}."
	body, defaults := substitutePositional(tmpl, args)
	if body != "Investigate $1/$2 over $3." {
		t.Errorf("unexpected body: %q", body)
	}
	if defaults["duration"] != "30m" {
		t.Errorf("expected duration default '30m', got %q", defaults["duration"])
	}
	if defaults["namespace"] != "" {
		t.Errorf("expected no default for required arg namespace")
	}
}

func TestSubstitutePositionalLeavesUnknownTokens(t *testing.T) {
	args := []rcmcp.PromptArgument{{Name: "x", Required: true}}
	body, _ := substitutePositional("hello {{unknown}} {{x}}", args)
	if body != "hello {{unknown}} $1" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestArgumentHint(t *testing.T) {
	hint := argumentHint([]rcmcp.PromptArgument{
		{Name: "namespace", Required: true},
		{Name: "workload", Required: true},
		{Name: "duration", Required: false},
	})
	if hint != "<namespace> <workload> [duration]" {
		t.Errorf("unexpected hint: %q", hint)
	}
}

func TestDefaultsSection(t *testing.T) {
	args := []rcmcp.PromptArgument{
		{Name: "namespace", Required: true},
		{Name: "workload", Required: true},
		{Name: "duration", Required: false},
	}
	defaults := map[string]string{"duration": "30m"}
	block := defaultsSection(args, defaults)
	if !strings.Contains(block, "$3 (duration) → 30m") {
		t.Errorf("defaults section missing expected line:\n%s", block)
	}
	// Empty defaults should produce no block.
	if defaultsSection(args, map[string]string{}) != "" {
		t.Errorf("expected empty defaults block when map empty")
	}
}

func TestCommandFileNameByFormat(t *testing.T) {
	cases := []struct {
		name   string
		format commandFormat
		want   string
	}{
		{"observability_workload_diagnose", formatClaudeCommand, "observability-workload-diagnose.md"},
		{"observability_workload_diagnose", formatCopilotPrompt, "observability-workload-diagnose.prompt.md"},
		{"observability_workload_diagnose", formatCursorCommand, "observability-workload-diagnose.md"},
		{"observability_workload_diagnose", formatGenericMD, "observability-workload-diagnose.md"},
	}
	for _, c := range cases {
		if got := commandFileName(c.name, c.format); got != c.want {
			t.Errorf("commandFileName(%q, %v) = %q, want %q", c.name, c.format, got, c.want)
		}
	}
}

func TestRenderCommandFileFrontMatter(t *testing.T) {
	spec := rcmcp.PromptSpec{
		Name:        "test_prompt",
		Description: "Hello world",
		Arguments: []rcmcp.PromptArgument{
			{Name: "namespace", Required: true},
			{Name: "duration", Required: false},
		},
		Template: "Run for {{namespace}} over {{duration|30m}}.",
	}
	out := renderCommandFile(spec, formatClaudeCommand)
	for _, must := range []string{
		"description: Hello world",
		"argument-hint: <namespace> [duration]",
		"$1",
		"$2",
		"$2 (duration) → 30m",
	} {
		if !strings.Contains(out, must) {
			t.Errorf("rendered file missing %q\n---\n%s", must, out)
		}
	}

	// Copilot format uses .prompt.md and mode: agent.
	out = renderCommandFile(spec, formatCopilotPrompt)
	if !strings.Contains(out, "mode: agent") {
		t.Errorf("copilot format missing 'mode: agent' header:\n%s", out)
	}
}

func TestSyncCommandsForTargetCreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	target := commandTargets["claude"]
	specs := []rcmcp.PromptSpec{
		{
			Name:        "alpha_beta",
			Description: "Test prompt",
			Arguments:   []rcmcp.PromptArgument{{Name: "namespace", Required: true}},
			Template:    "Hello {{namespace}}",
		},
	}
	count, dest, err := syncCommandsForTarget(tmp, target, specs, true, false)
	if err != nil {
		t.Fatalf("syncCommandsForTarget: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 file synced, got %d", count)
	}
	expected := filepath.Join(dest, "alpha-beta.md")
	body, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file %s: %v", expected, err)
	}
	if !strings.Contains(string(body), "Hello $1") {
		t.Errorf("file body missing substitution:\n%s", body)
	}
}

func TestSyncCommandsDryRunDoesNotWrite(t *testing.T) {
	tmp := t.TempDir()
	target := commandTargets["claude"]
	specs := []rcmcp.PromptSpec{
		{Name: "x", Description: "test", Template: "static"},
	}
	count, dest, err := syncCommandsForTarget(tmp, target, specs, true, true)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected dry-run count=1, got %d", count)
	}
	if _, err := os.Stat(filepath.Join(dest, "x.md")); err == nil {
		t.Errorf("dry run should not have written file")
	}
}

func TestSyncCommandsOverwriteFalseSkipsExisting(t *testing.T) {
	tmp := t.TempDir()
	target := commandTargets["claude"]
	specs := []rcmcp.PromptSpec{
		{Name: "x", Description: "first", Template: "first"},
	}
	if _, _, err := syncCommandsForTarget(tmp, target, specs, true, false); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	specs[0].Template = "second"
	count, dest, err := syncCommandsForTarget(tmp, target, specs, false, false)
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 writes when overwrite=false and file exists, got %d", count)
	}
	body, _ := os.ReadFile(filepath.Join(dest, "x.md"))
	if !strings.Contains(string(body), "first") {
		t.Errorf("existing file should remain untouched: %s", body)
	}
}

func TestSelectedPromptsFilter(t *testing.T) {
	all := []rcmcp.PromptSpec{
		{Name: "alpha"}, {Name: "beta"}, {Name: "gamma"},
	}
	out, err := selectedPrompts(all, []string{"beta", "GAMMA"})
	if err != nil {
		t.Fatalf("selectedPrompts: %v", err)
	}
	if len(out) != 2 || out[0].Name != "beta" || out[1].Name != "gamma" {
		t.Errorf("unexpected filtered result: %+v", out)
	}
	if _, err := selectedPrompts(all, []string{"nonexistent"}); err == nil {
		t.Errorf("expected error for unknown filter")
	}
}

func TestExpandPromptHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	if got, _ := expandPromptHome("~"); got != home {
		t.Errorf("~ -> %q, want %q", got, home)
	}
	if got, _ := expandPromptHome("~/foo/bar"); got != filepath.Join(home, "foo/bar") {
		t.Errorf("~/foo/bar -> %q", got)
	}
	if got, _ := expandPromptHome("/tmp/x"); got != "/tmp/x" {
		t.Errorf("absolute path mangled: %q", got)
	}
	if got, _ := expandPromptHome(""); got != "." {
		t.Errorf("empty path -> %q", got)
	}
}
