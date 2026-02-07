package k8s

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestCommandIsShell(t *testing.T) {
	if !commandIsShell([]string{"bash"}) {
		t.Fatalf("expected bash to be shell")
	}
	if commandIsShell([]string{"echo"}) {
		t.Fatalf("expected echo to be non-shell")
	}
}

func TestHandleExecMissingArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	_, err := toolset.handleExec(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err == nil {
		t.Fatalf("expected error for missing args")
	}
}

func TestHandleExecShellBlocked(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	_, err := toolset.handleExec(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api-1",
			"command":   []any{"sh"},
		},
	})
	if err == nil {
		t.Fatalf("expected shell command error")
	}
}
