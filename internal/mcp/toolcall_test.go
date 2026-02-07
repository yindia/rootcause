package mcp

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/policy"
)

func TestToolContextCallToolMissingInvoker(t *testing.T) {
	ctx := ToolContext{}
	_, err := ctx.CallTool(context.Background(), policy.User{}, "demo", nil)
	if err == nil {
		t.Fatalf("expected error for missing invoker")
	}
}

func TestToolContextCallTool(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	toolCtx := ToolContext{Config: &cfg, Policy: policy.NewAuthorizer(), Registry: reg}
	toolCtx.Invoker = NewToolInvoker(reg, toolCtx)

	result, err := toolCtx.CallTool(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["ok"] != true {
		t.Fatalf("unexpected tool result: %#v", result.Data)
	}
}
