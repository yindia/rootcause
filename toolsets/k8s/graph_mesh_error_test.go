package k8s

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestAddMeshGraphDiscoveryError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{},
		Policy:  policy.NewAuthorizer(),
	})
	cache := newGraphCache()
	cache.servicesLoaded = true
	graph := newGraphBuilder()
	warnings := toolset.addMeshGraph(context.Background(), graph, "default", cache)
	if len(warnings) == 0 {
		t.Fatalf("expected mesh graph warnings with missing discovery client")
	}
}
