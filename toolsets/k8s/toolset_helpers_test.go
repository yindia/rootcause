package k8s

import (
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestToolsetHelpers(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	if toolset.Version() == "" {
		t.Fatalf("expected version")
	}
	ctx := toolset.toolContext()
	if ctx.Config == nil {
		t.Fatalf("expected tool context config")
	}
	if err := toolset.checkAllowedNamespace([]string{"default"}, "kube-system"); err == nil {
		t.Fatalf("expected namespace check error")
	}
	if resourceRef("pods", "default", "api") != "pods/default/api" {
		t.Fatalf("unexpected resource ref")
	}
}
