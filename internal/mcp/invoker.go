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
	ctx = ensureTraceContext(ctx)
	if i == nil || i.reg == nil {
		err := errors.New("tool registry not available")
		return ToolResult{Data: BuildErrorEnvelope(err, nil)}, err
	}
	spec, ok := i.reg.Get(toolName)
	if !ok {
		err := errors.New("tool not found")
		return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": toolName})}, err
	}
	if i.ctx.Policy != nil {
		if err := i.ctx.Policy.AuthorizeTool(user, spec.ToolsetID, spec.Name); err != nil {
			logAudit(ctx, i.ctx, spec, user.ID, nil, nil, "error", err)
			return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "toolset": spec.ToolsetID})}, err
		}
		namespace, namespaced := inferNamespaceScope(spec, args)
		if err := i.ctx.Policy.CheckNamespace(user, namespace, namespaced); err != nil {
			namespaces := []string{}
			if namespace != "" {
				namespaces = append(namespaces, namespace)
			}
			logAudit(ctx, i.ctx, spec, user.ID, namespaces, nil, "error", err)
			return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "namespace": namespace, "namespaced": namespaced})}, err
		}
	}
	if i.ctx.Clients != nil && i.ctx.Config != nil {
		ttl := time.Duration(i.ctx.Config.Cache.DiscoveryTTLSeconds) * time.Second
		i.ctx.Clients.RefreshDiscovery(ttl)
	}
	if op, ok := preflightOperationForTool(spec.Name); ok {
		if preflightErr := i.runMutationPreflight(ctx, user, args, op); preflightErr != nil {
			logAudit(ctx, i.ctx, spec, user.ID, nil, nil, "error", preflightErr)
			return ToolResult{Data: BuildErrorEnvelope(preflightErr, map[string]any{"tool": spec.Name, "operation": op})}, preflightErr
		}
	}
	chain, _ := callChainFromContext(ctx)
	if len(chain) > 0 {
		parent := chain[len(chain)-1]
		i.ctx.CallGraph.Record(parent, spec.Name)
	}
	chain = append(chain, spec.Name)
	execCtx := withCallChain(ctx, chain)
	execCtx, cancel := withToolTimeout(execCtx, i.ctx.Config, spec)
	result, toolErr := spec.Handler(execCtx, ToolRequest{Arguments: args, User: user, Context: i.ctx})
	cancel()
	outcome := "success"
	if toolErr != nil {
		outcome = "error"
		result.Data = canonicalErrorPayload(toolErr, result.Data)
	}
	logAudit(execCtx, i.ctx, spec, user.ID, result.Metadata.Namespaces, result.Metadata.Resources, outcome, toolErr)
	return result, toolErr
}

func canonicalErrorPayload(err error, details any) map[string]any {
	if root, ok := details.(map[string]any); ok {
		if isErrorEnvelope(root) {
			return root
		}
	}
	return BuildErrorEnvelope(err, details)
}

func isErrorEnvelope(payload map[string]any) bool {
	errorObj, ok := payload["error"].(map[string]any)
	if !ok {
		return false
	}
	_, hasCode := errorObj["code"].(string)
	_, hasMessage := errorObj["message"].(string)
	return hasCode && hasMessage
}

func inferNamespaceScope(spec ToolSpec, args map[string]any) (string, bool) {
	if namespace, ok := args["namespace"].(string); ok {
		return namespace, true
	}
	if spec.InputSchema != nil {
		if props, ok := spec.InputSchema["properties"].(map[string]any); ok {
			if _, exists := props["namespace"]; exists {
				return "", true
			}
		}
	}
	return "", false
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
