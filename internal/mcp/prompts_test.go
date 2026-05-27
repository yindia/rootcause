package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/config"
)

func TestRenderPromptTemplate(t *testing.T) {
	text := renderPromptTemplate("hello {{name}} in {{namespace|default}}", map[string]string{"name": "api"})
	if text != "hello api in default" {
		t.Fatalf("unexpected rendered prompt: %q", text)
	}
}

func TestGCPWorkloadDiagnoseRendering(t *testing.T) {
	var spec promptSpec
	for _, p := range builtinPrompts {
		if p.Name == "gcp_workload_diagnose" {
			spec = p
			break
		}
	}
	if spec.Name == "" {
		t.Fatalf("expected gcp_workload_diagnose to be defined")
	}
	if len(spec.Arguments) < 2 {
		t.Fatalf("expected at least namespace + workload args")
	}
	h := buildPromptHandler(spec)
	res, err := h(context.Background(), &sdkmcp.GetPromptRequest{Params: &sdkmcp.GetPromptParams{Arguments: map[string]string{
		"namespace":  "payments",
		"workload":   "checkout-api",
		"project_id": "my-obs-proj",
		"duration":   "1h",
	}}})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	text := res.Messages[0].Content.(*sdkmcp.TextContent).Text
	for _, must := range []string{
		"payments/checkout-api",
		"Project: my-obs-proj",
		"Window: 1h",
		"rootcause.incident_bundle",
		"gcp.logs.error_timeline",
		"gcp.logs.correlated_with_bundle",
		"gcp.metrics.slo_list",
	} {
		if !strings.Contains(text, must) {
			t.Errorf("rendered prompt missing %q\n--- rendered ---\n%s", must, text)
		}
	}
}

func TestBuiltinPromptHandler(t *testing.T) {
	h := buildPromptHandler(promptSpec{
		Name:        "test",
		Description: "desc",
		Template:    "debug {{workload}} in {{namespace|default}}",
	})
	res, err := h(context.Background(), &sdkmcp.GetPromptRequest{Params: &sdkmcp.GetPromptParams{Arguments: map[string]string{"workload": "payments"}}})
	if err != nil {
		t.Fatalf("prompt handler: %v", err)
	}
	if len(res.Messages) != 1 {
		t.Fatalf("expected one prompt message")
	}
	content, ok := res.Messages[0].Content.(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected text content")
	}
	if content.Text != "debug payments in default" {
		t.Fatalf("unexpected prompt text: %q", content.Text)
	}
}

func TestRegisterSDKPrompts(t *testing.T) {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rootcause", Version: "test"}, nil)
	names, err := RegisterSDKPrompts(server, ToolContext{})
	if err != nil {
		t.Fatalf("register prompts: %v", err)
	}
	if len(names) < 15 {
		t.Fatalf("expected built-in prompts to be registered")
	}
	required := map[string]bool{
		"troubleshoot_workload":     false,
		"sre_incident_commander":    false,
		"istio_mesh_diagnose":       false,
		"terraform_drift_triage":    false,
		"aws_eks_operational_check": false,
		"gcp_workload_diagnose":     false,
	}
	for _, name := range names {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Fatalf("expected prompt %s to be registered", name)
		}
	}
}

func TestLoadPromptSpecsFromTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.toml")
	content := `
[[prompt]]
name = "custom_incident"
title = "Custom Incident"
description = "Detailed incident flow"
template = "Investigate {{service|payments}}"

  [[prompt.arguments]]
  name = "service"
  description = "Service name"
  required = false
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}
	specs, err := loadPromptSpecsFromTOML(path)
	if err != nil {
		t.Fatalf("loadPromptSpecsFromTOML: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected one prompt, got %d", len(specs))
	}
	if specs[0].Name != "custom_incident" || specs[0].Title != "Custom Incident" {
		t.Fatalf("unexpected prompt spec: %#v", specs[0])
	}
	if len(specs[0].Arguments) != 1 || specs[0].Arguments[0].Name != "service" {
		t.Fatalf("unexpected prompt arguments: %#v", specs[0].Arguments)
	}
}

func TestSplitFrontMatter(t *testing.T) {
	body := []byte("---\nname: hello\ndescription: world\n---\n\nThe body\nover two lines\n")
	front, rest, err := splitFrontMatter(body)
	if err != nil {
		t.Fatalf("splitFrontMatter: %v", err)
	}
	if !strings.Contains(string(front), "name: hello") {
		t.Errorf("front-matter not extracted: %q", front)
	}
	if !strings.HasPrefix(string(rest), "The body") {
		t.Errorf("body not extracted: %q", rest)
	}

	// File with no front-matter returns body unchanged.
	plain := []byte("just a body, no front-matter\n")
	front, rest, err = splitFrontMatter(plain)
	if err != nil {
		t.Fatalf("splitFrontMatter plain: %v", err)
	}
	if front != nil {
		t.Errorf("expected nil front-matter on plain body, got %q", front)
	}
	if string(rest) != string(plain) {
		t.Errorf("plain body should pass through, got %q", rest)
	}

	// Unclosed front-matter is an error.
	if _, _, err := splitFrontMatter([]byte("---\nname: oops\n")); err == nil {
		t.Errorf("expected error for unclosed front-matter")
	}

	// BOM at start is tolerated.
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, body...)
	front, _, err = splitFrontMatter(withBOM)
	if err != nil {
		t.Fatalf("BOM should be stripped, got error: %v", err)
	}
	if !strings.Contains(string(front), "name: hello") {
		t.Errorf("BOM stripped but front-matter missing: %q", front)
	}
}

func TestLoadPromptSpecFromMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team-status.md")
	content := `---
name: team_status
description: Daily status check
arguments:
  - name: workload
    description: Deployment name
    required: true
  - name: namespace
    description: Namespace
    required: false
---

Show me {{workload}} in {{namespace|default}}.
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	spec, err := loadPromptSpecFromMarkdown(path)
	if err != nil {
		t.Fatalf("loadPromptSpecFromMarkdown: %v", err)
	}
	if spec.Name != "team_status" {
		t.Errorf("expected name team_status, got %q", spec.Name)
	}
	if spec.Description != "Daily status check" {
		t.Errorf("unexpected description: %q", spec.Description)
	}
	if len(spec.Arguments) != 2 {
		t.Fatalf("expected 2 arguments, got %d", len(spec.Arguments))
	}
	if spec.Arguments[0].Name != "workload" || !spec.Arguments[0].Required {
		t.Errorf("workload arg malformed: %#v", spec.Arguments[0])
	}
	if spec.Arguments[1].Required {
		t.Errorf("namespace arg should be optional")
	}
	if !strings.HasPrefix(spec.Template, "Show me {{workload}}") {
		t.Errorf("template not preserved: %q", spec.Template)
	}
}

func TestLoadPromptSpecFromMarkdownFallsBackToFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-special-prompt.md")
	// No `name` in front-matter; loader should derive from file name.
	content := `---
description: No explicit name
---

Hello {{x}}.
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	spec, err := loadPromptSpecFromMarkdown(path)
	if err != nil {
		t.Fatalf("loadPromptSpecFromMarkdown: %v", err)
	}
	if spec.Name != "my_special_prompt" {
		t.Errorf("expected derived name 'my_special_prompt', got %q", spec.Name)
	}
}

func TestLoadPromptSpecFromMarkdownEmptyBodyErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	content := `---
name: empty
description: Nothing here
---

`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := loadPromptSpecFromMarkdown(path); err == nil {
		t.Errorf("expected error for empty body")
	}
}

func TestLoadPromptSpecsFromDirMixesMDAndTOML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte(`---
name: a_prompt
description: alpha
arguments: []
---

Body of a.
`), 0o600); err != nil {
		t.Fatalf("write a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.toml"), []byte(`
[[prompt]]
name = "b_prompt"
description = "beta"
template = "Body of b."
`), 0o600); err != nil {
		t.Fatalf("write b.toml: %v", err)
	}
	// Files starting with dot are skipped.
	_ = os.WriteFile(filepath.Join(dir, ".hidden.md"), []byte(`---
name: should_not_load
description: hidden
---
hidden body`), 0o600)
	// Unknown extensions are skipped.
	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600)

	specs, _, err := loadPromptSpecsFromDir(dir)
	if err != nil {
		t.Fatalf("loadPromptSpecsFromDir: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 prompts (a + b), got %d (%+v)", len(specs), specs)
	}
	names := []string{specs[0].Name, specs[1].Name}
	if names[0] != "a_prompt" || names[1] != "b_prompt" {
		t.Errorf("unexpected alphabetical ordering: %v", names)
	}
}

func TestLoadPromptSpecsFromDirSkipsBadFilesWithWarning(t *testing.T) {
	dir := t.TempDir()
	// Valid prompt.
	_ = os.WriteFile(filepath.Join(dir, "good.md"), []byte("---\nname: good\ndescription: ok\n---\n\nBody.\n"), 0o600)
	// Malformed: empty body.
	_ = os.WriteFile(filepath.Join(dir, "empty.md"), []byte("---\nname: empty\ndescription: x\n---\n\n"), 0o600)
	// Malformed: no front-matter (stray markdown).
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Just notes, not a prompt\n"), 0o600)
	// Malformed: unterminated front-matter.
	_ = os.WriteFile(filepath.Join(dir, "broken.md"), []byte("---\nname: broken\n"), 0o600)

	specs, warnings, err := loadPromptSpecsFromDir(dir)
	if err != nil {
		t.Fatalf("loadPromptSpecsFromDir must not hard-fail on bad files: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "good" {
		t.Fatalf("expected only the valid prompt to load, got %+v", specs)
	}
	if len(warnings) != 3 {
		t.Fatalf("expected 3 skip warnings (empty, README, broken), got %d: %v", len(warnings), warnings)
	}
}

func TestLoadPromptSpecsFromDirWarnsOnDuplicateName(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nname: dup\ndescription: first\n---\n\nA.\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nname: dup\ndescription: second\n---\n\nB.\n"), 0o600)

	specs, warnings, err := loadPromptSpecsFromDir(dir)
	if err != nil {
		t.Fatalf("loadPromptSpecsFromDir: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected 1 prompt after dedupe, got %d", len(specs))
	}
	foundDupWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "duplicate prompt name") {
			foundDupWarning = true
		}
	}
	if !foundDupWarning {
		t.Errorf("expected a duplicate-name warning, got: %v", warnings)
	}
}

func TestLoadPromptSpecsHonorsDirThenLegacyFile(t *testing.T) {
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "from-dir.md"), []byte(`---
name: from_dir
description: loaded from dir
---

Body.
`), 0o600); err != nil {
		t.Fatalf("write dir prompt: %v", err)
	}
	legacy := filepath.Join(dir, "legacy.toml")
	if err := os.WriteFile(legacy, []byte(`
[[prompt]]
name = "from_legacy"
description = "loaded from legacy file"
template = "Hello"
`), 0o600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	t.Setenv("ROOTCAUSE_PROMPTS_DIR", promptsDir)
	t.Setenv("ROOTCAUSE_PROMPTS_FILE", legacy)

	specs, err := loadPromptSpecs(ToolContext{})
	if err != nil {
		t.Fatalf("loadPromptSpecs: %v", err)
	}
	gotDir := false
	gotLegacy := false
	for _, s := range specs {
		if s.Name == "from_dir" {
			gotDir = true
		}
		if s.Name == "from_legacy" {
			gotLegacy = true
		}
	}
	if !gotDir {
		t.Errorf("expected dir prompt to be loaded")
	}
	if !gotLegacy {
		t.Errorf("expected legacy file prompt to be loaded alongside dir")
	}
}

func TestLoadPromptSpecsFromTOMLLegacyArgumentKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.toml")
	content := `
[[prompt]]
name = "legacy_prompt"
description = "Legacy argument key"
template = "Investigate {{service|payments}}"

  [[prompt.argument]]
  name = "service"
  description = "Service name"
  required = false
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}
	specs, err := loadPromptSpecsFromTOML(path)
	if err != nil {
		t.Fatalf("loadPromptSpecsFromTOML: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("expected one prompt, got %d", len(specs))
	}
	if len(specs[0].Arguments) != 1 || specs[0].Arguments[0].Name != "service" {
		t.Fatalf("unexpected prompt arguments: %#v", specs[0].Arguments)
	}
}

func TestLoadPromptSpecs_CustomOverridesBuiltin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.toml")
	content := `
[[prompt]]
name = "security_audit"
description = "Custom security workflow"
template = "Custom security audit for {{namespace|team-a}}"
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}
	t.Setenv("MCP_PROMPTS_FILE", path)
	specs, err := loadPromptSpecs(ToolContext{})
	if err != nil {
		t.Fatalf("loadPromptSpecs: %v", err)
	}
	var found bool
	for _, spec := range specs {
		if spec.Name == "security_audit" {
			found = true
			if spec.Description != "Custom security workflow" {
				t.Fatalf("expected override description, got %q", spec.Description)
			}
			if spec.Template != "Custom security audit for {{namespace|team-a}}" {
				t.Fatalf("expected override template, got %q", spec.Template)
			}
		}
	}
	if !found {
		t.Fatalf("expected overridden security_audit prompt")
	}
}

func TestResolvePromptConfigPath_FromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompts.toml")
	if err := os.WriteFile(path, []byte("[[prompt]]\nname='x'\ntemplate='y'\n"), 0600); err != nil {
		t.Fatalf("write prompt config: %v", err)
	}
	t.Setenv("MCP_PROMPTS_FILE", "")
	t.Setenv("ROOTCAUSE_PROMPTS_FILE", "")
	ctx := ToolContext{Config: &config.Config{Prompts: config.PromptsConfig{File: path}}}
	resolved := resolvePromptConfigPath(ctx)
	if resolved != path {
		t.Fatalf("expected config prompts file path, got %q", resolved)
	}
}
