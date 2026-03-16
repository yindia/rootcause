package mcp

import (
	"context"
	"errors"

	"rootcause/internal/policy"
)

func (t ToolContext) CallTool(ctx context.Context, user policy.User, toolName string, args map[string]any) (ToolResult, error) {
	if t.Invoker == nil {
		err := errors.New("tool invoker not available")
		return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": toolName})}, err
	}
	return t.Invoker.Call(ctx, user, toolName, args)
}
