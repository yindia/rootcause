package mcp

import (
	"context"
	"errors"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/policy"
)

func TestInvokerToolNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	invoker := NewToolInvoker(reg, ToolContext{})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "missing", nil)
	if err == nil {
		t.Fatalf("expected error for missing tool")
	}
}

func TestInvokerHandlerError(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"error": "fail"}}, errors.New("fail")
		},
	})
	ctx := ToolContext{Policy: policy.NewAuthorizer()}
	invoker := NewToolInvoker(reg, ctx)
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected handler error")
	}
}

func TestInvokerMissingRegistry(t *testing.T) {
	invoker := &ToolInvoker{}
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected error for missing registry")
	}
}

func TestInvokerSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer()})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatalf("expected result data")
	}
}
