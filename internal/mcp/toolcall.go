package mcp

import (
	"context"
	"errors"

	"rootcause/internal/policy"
)

func (t ToolContext) CallTool(ctx context.Context, user policy.User, toolName string, args map[string]any) (ToolResult, error) {
	if t.Invoker == nil {
		return ToolResult{Data: map[string]any{"error": "tool invoker not available"}}, errors.New("tool invoker not available")
	}
	return t.Invoker.Call(ctx, user, toolName, args)
}
