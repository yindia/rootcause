package aws

import (
	"context"
	"testing"

	"rootcause/internal/cache"
	"rootcause/internal/config"
	"rootcause/internal/mcp"
)

func TestStableValue(t *testing.T) {
	value := map[string]any{
		"b": "two",
		"a": []any{"x", "y"},
		"c": map[string]string{"k": "v"},
	}
	got := stableValue(value)
	want := "{a=[x,y],b=two,c={k=v}}"
	if got != want {
		t.Fatalf("unexpected stableValue: %q", got)
	}
	if got := stableValue("  trim "); got != "trim" {
		t.Fatalf("expected trimmed string, got %q", got)
	}
}

func TestAWSListCacheKey(t *testing.T) {
	key := awsListCacheKey("aws.ec2.list_instances", map[string]any{"region": "us-east-1"})
	if key == "" || key == "aws.ec2.list_instances" {
		t.Fatalf("expected cache key, got %q", key)
	}
}

func TestWrapListCacheNoCache(t *testing.T) {
	ctx := mcp.ToolsetContext{}
	toolset := &Toolset{ctx: ctx}
	calls := 0
	spec := mcp.ToolSpec{
		Name: "aws.ec2.list_instances",
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			calls++
			return mcp.ToolResult{}, nil
		},
	}
	wrapped := toolset.wrapListCache(spec)
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	if calls != 2 {
		t.Fatalf("expected handler to run without cache")
	}
}

func TestWrapListCacheNonList(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cache.AWSListTTLSeconds = 60
	ctx := mcp.ToolsetContext{
		Config: &cfg,
		Cache:  cache.NewStore(),
	}
	toolset := &Toolset{ctx: ctx}
	calls := 0
	spec := mcp.ToolSpec{
		Name: "aws.ec2.get_instance",
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			calls++
			return mcp.ToolResult{}, nil
		},
	}
	wrapped := toolset.wrapListCache(spec)
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	if calls != 2 {
		t.Fatalf("expected handler to run without caching")
	}
}

func TestWrapListCacheDisabledTTL(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Cache.AWSListTTLSeconds = 0
	ctx := mcp.ToolsetContext{
		Config: &cfg,
		Cache:  cache.NewStore(),
	}
	toolset := &Toolset{ctx: ctx}
	calls := 0
	spec := mcp.ToolSpec{
		Name: "aws.ec2.list_instances",
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			calls++
			return mcp.ToolResult{}, nil
		},
	}
	wrapped := toolset.wrapListCache(spec)
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	_, _ = wrapped.Handler(context.Background(), mcp.ToolRequest{})
	if calls != 2 {
		t.Fatalf("expected handler to run without caching when TTL disabled")
	}
}
