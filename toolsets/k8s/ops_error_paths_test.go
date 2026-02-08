package k8s

import (
	"context"
	"testing"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestOpsHandlersErrorPaths(t *testing.T) {
	toolset, _ := newTestToolset()

	if _, err := toolset.handleGet(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod"},
	}); err == nil {
		t.Fatalf("expected handleGet missing name error")
	}

	if _, err := toolset.handleGet(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod", "name": "demo"},
	}); err == nil {
		t.Fatalf("expected handleGet missing namespace error")
	}

	if _, err := toolset.handleList(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected handleList resources list error")
	}

	if _, err := toolset.handleDescribe(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod"},
	}); err == nil {
		t.Fatalf("expected handleDescribe missing name error")
	}

	if _, err := toolset.handleDelete(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod", "name": "demo", "namespace": "default"},
	}); err == nil {
		t.Fatalf("expected handleDelete confirm error")
	}

	if _, err := toolset.handlePatch(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod", "name": "demo", "namespace": "default", "confirm": true},
	}); err == nil {
		t.Fatalf("expected handlePatch missing patch error")
	}
}
