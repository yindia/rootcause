package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	if i.ctx.Clients != nil && i.ctx.Config != nil {
		ttl := time.Duration(i.ctx.Config.Cache.DiscoveryTTLSeconds) * time.Second
		i.ctx.Clients.RefreshDiscovery(ttl)
	}
	if op, ok := preflightOperationForTool(spec.Name); ok {
		if preflightErr := i.runMutationPreflight(ctx, user, args, op); preflightErr != nil {
			logAudit(i.ctx, spec, user.ID, nil, nil, "error", preflightErr)
			return ToolResult{Data: map[string]any{"error": preflightErr.Error()}}, preflightErr
		}
	}
	execCtx, cancel := withToolTimeout(ctx, i.ctx.Config, spec)
	result, toolErr := spec.Handler(execCtx, ToolRequest{Arguments: args, User: user, Context: i.ctx})
	cancel()
	outcome := "success"
	if toolErr != nil {
		outcome = "error"
	}
	logAudit(i.ctx, spec, user.ID, result.Metadata.Namespaces, result.Metadata.Resources, outcome, toolErr)
	return result, toolErr
}

func preflightOperationForTool(toolName string) (string, bool) {
	switch toolName {
	case "k8s.create", "kubectl_create":
		return "create", true
	case "k8s.apply", "kubectl_apply":
		return "apply", true
	case "k8s.patch", "kubectl_patch":
		return "patch", true
	case "k8s.delete", "kubectl_delete":
		return "delete", true
	case "k8s.scale", "kubectl_scale":
		return "scale", true
	case "k8s.rollout", "kubectl_rollout":
		return "rollout", true
	case "k8s.cleanup_pods":
		return "cleanup_pods", true
	case "k8s.node_management":
		return "node_management", true
	default:
		return "", false
	}
}

func (i *ToolInvoker) runMutationPreflight(ctx context.Context, user policy.User, args map[string]any, operation string) error {
	if i == nil || i.reg == nil {
		return nil
	}
	preflight, ok := i.reg.Get("k8s.safe_mutation_preflight")
	if !ok {
		return errors.New("k8s.safe_mutation_preflight tool is required for mutating operations")
	}
	preflightArgs := map[string]any{}
	for key, value := range args {
		preflightArgs[key] = value
	}
	preflightArgs["operation"] = operation
	result, err := preflight.Handler(ctx, ToolRequest{Arguments: preflightArgs, User: user, Context: i.ctx})
	if err != nil {
		return err
	}
	root, ok := result.Data.(map[string]any)
	if !ok {
		return errors.New("mutation preflight returned invalid response")
	}
	safe, ok := root["safeToProceed"].(bool)
	if !ok {
		return errors.New("mutation preflight response missing safeToProceed")
	}
	if !safe {
		if summary, ok := root["summary"].(map[string]any); ok {
			return fmt.Errorf("mutation preflight failed for operation %s: %v", operation, summary)
		}
		return fmt.Errorf("mutation preflight failed for operation %s", operation)
	}
	return nil
}
