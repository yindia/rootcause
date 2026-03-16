package mcp

import (
	"context"
	"os"
	"path/filepath"
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
	if len(names) < 8 {
		t.Fatalf("expected built-in prompts to be registered")
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
	if specs[0].Name != "custom_incident" || specs[0].Title != "Custom Incident" {
		t.Fatalf("unexpected prompt spec: %#v", specs[0])
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
