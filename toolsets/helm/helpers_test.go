package helm

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestExpandHome(t *testing.T) {
	got := expandHome("~/.kube/config")
	if got == "" || got == "~/.kube/config" {
		t.Fatalf("expected expanded home path, got %q", got)
	}
	if same := expandHome("/tmp/config"); same != "/tmp/config" {
		t.Fatalf("expected unchanged path, got %q", same)
	}
}

func TestSharedRESTClientGetterErrors(t *testing.T) {
	getter := &sharedRESTClientGetter{}
	if _, err := getter.ToRESTConfig(); err == nil {
		t.Fatalf("expected rest config error")
	}
	if _, err := getter.ToDiscoveryClient(); err == nil {
		t.Fatalf("expected discovery error")
	}
	if _, err := getter.ToRESTMapper(); err == nil {
		t.Fatalf("expected rest mapper error")
	}
}

func TestHelmSettingsFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = "/tmp/kubeconfig"
	cfg.Context = "demo"
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg}}
	settings := toolset.helmSettings()
	if settings.KubeConfig != cfg.Kubeconfig || settings.KubeContext != cfg.Context {
		t.Fatalf("unexpected helm settings: %#v", settings)
	}
}

func TestTemplateApplyInvalidManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := &Toolset{ctx: mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}}}
	_, err := toolset.handleTemplateApply(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{
			"release":   "demo",
			"namespace": "default",
			"manifest":  "not-yaml: [",
			"confirm":   true,
		},
	})
	if err == nil {
		t.Fatalf("expected template apply error")
	}
}
