package aws

import (
	"context"
	"testing"

	"rootcause/internal/cache"
	"rootcause/internal/config"
	"rootcause/internal/mcp"
)

func TestWrapListCache(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cache.AWSListTTLSeconds = 60
	ctx := mcp.ToolsetContext{
		Config: &cfg,
		Cache:  cache.NewStore(),
	}
	toolset := &Toolset{ctx: ctx}

	calls := 0
	spec := mcp.ToolSpec{
		Name:      "aws.ec2.list_instances",
		ToolsetID: "aws",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			calls++
			return mcp.ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	}

	wrapped := toolset.wrapListCache(spec)
	_, err := wrapped.Handler(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"region": "us-east-1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = wrapped.Handler(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"region": "us-east-1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected cached call, got %d", calls)
	}
}
