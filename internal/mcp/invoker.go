package mcp

import (
	"context"
	"errors"

	"rootcause/internal/policy"
)

type ToolInvoker struct {
	reg *ToolRegistry
	ctx ToolContext
}

func NewToolInvoker(reg *ToolRegistry, ctx ToolContext) *ToolInvoker {
	return &ToolInvoker{reg: reg, ctx: ctx}
}

func (i *ToolInvoker) Call(ctx context.Context, user policy.User, toolName string, args map[string]any) (ToolResult, error) {
	if i == nil || i.reg == nil {
		return ToolResult{Data: map[string]any{"error": "tool registry not available"}}, errors.New("tool registry not available")
	}
	spec, ok := i.reg.Get(toolName)
	if !ok {
		return ToolResult{Data: map[string]any{"error": "tool not found"}}, errors.New("tool not found")
	}
	if i.ctx.Policy != nil {
		if err := i.ctx.Policy.AuthorizeTool(user, spec.ToolsetID, spec.Name); err != nil {
			logAudit(i.ctx, spec, user.ID, nil, nil, "error", err)
			return ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	result, toolErr := spec.Handler(ctx, ToolRequest{Arguments: args, User: user, Context: i.ctx})
	outcome := "success"
	if toolErr != nil {
		outcome = "error"
	}
	logAudit(i.ctx, spec, user.ID, result.Metadata.Namespaces, result.Metadata.Resources, outcome, toolErr)
	return result, toolErr
}
