package mcp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync/atomic"
	"time"

	"rootcause/internal/config"
	"rootcause/internal/policy"
)

type invokerRuntime struct {
	reg *ToolRegistry
	ctx ToolContext
}

type ToolInvoker struct {
	rt         atomic.Pointer[invokerRuntime]
	skillCache atomic.Pointer[customSkillCache]
}

func NewToolInvoker(reg *ToolRegistry, ctx ToolContext) *ToolInvoker {
	inv := &ToolInvoker{}
	inv.rt.Store(&invokerRuntime{reg: reg, ctx: ctx})
	inv.skillCache.Store(newCustomSkillCache())
	return inv
}

func (i *ToolInvoker) Swap(reg *ToolRegistry, ctx ToolContext) {
	if i == nil {
		return
	}
	i.rt.Store(&invokerRuntime{reg: reg, ctx: ctx})
	i.skillCache.Store(newCustomSkillCache())
}

func (i *ToolInvoker) Runtime() (*ToolRegistry, ToolContext, bool) {
	if i == nil {
		return nil, ToolContext{}, false
	}
	rt := i.rt.Load()
	if rt == nil {
		return nil, ToolContext{}, false
	}
	return rt.reg, rt.ctx, true
}

func (i *ToolInvoker) Call(ctx context.Context, user policy.User, toolName string, args map[string]any) (ToolResult, error) {
	ctx = ensureTraceContext(ctx)
	if i == nil {
		err := errors.New("tool invoker not available")
		return ToolResult{Data: BuildErrorEnvelope(err, nil)}, err
	}
	rt := i.rt.Load()
	if rt == nil || rt.reg == nil {
		err := errors.New("tool registry not available")
		return ToolResult{Data: BuildErrorEnvelope(err, nil)}, err
	}
	tctx := rt.ctx
	spec, ok := rt.reg.Get(toolName)
	if !ok {
		err := errors.New("tool not found")
		return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": toolName})}, err
	}
	chain, _ := callChainFromContext(ctx)
	if maxDepth := maxCallDepth(tctx.Config); maxDepth > 0 && len(chain) >= maxDepth {
		err := fmt.Errorf("call depth %d exceeds max %d at tool %s", len(chain), maxDepth, spec.Name)
		logAudit(ctx, tctx, spec, user.ID, nil, nil, "error", err)
		return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "chain": chain})}, err
	}
	if slices.Contains(chain, spec.Name) {
		err := fmt.Errorf("call cycle detected: %s already in chain %v", spec.Name, chain)
		logAudit(ctx, tctx, spec, user.ID, nil, nil, "error", err)
		return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "chain": chain})}, err
	}
	if tctx.Policy != nil {
		if err := tctx.Policy.AuthorizeTool(user, spec.ToolsetID, spec.Name); err != nil {
			logAudit(ctx, tctx, spec, user.ID, nil, nil, "error", err)
			return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "toolset": spec.ToolsetID})}, err
		}
		namespace, namespaced := inferNamespaceScope(spec, args)
		if err := tctx.Policy.CheckNamespace(user, namespace, namespaced); err != nil {
			namespaces := []string{}
			if namespace != "" {
				namespaces = append(namespaces, namespace)
			}
			logAudit(ctx, tctx, spec, user.ID, namespaces, nil, "error", err)
			return ToolResult{Data: BuildErrorEnvelope(err, map[string]any{"tool": spec.Name, "namespace": namespace, "namespaced": namespaced})}, err
		}
	}
	if tctx.Clients != nil && tctx.Config != nil {
		ttl := time.Duration(tctx.Config.Cache.DiscoveryTTLSeconds) * time.Second
		tctx.Clients.RefreshDiscovery(ttl)
	}
	if spec.Preflight != nil {
		if preflightErr := i.runMutationPreflight(ctx, rt, user, args, spec.Preflight); preflightErr != nil {
			logAudit(ctx, tctx, spec, user.ID, nil, nil, "error", preflightErr)
			return ToolResult{Data: BuildErrorEnvelope(preflightErr, map[string]any{"tool": spec.Name, "operation": spec.Preflight.Operation})}, preflightErr
		}
	}
	if len(chain) > 0 {
		parent := chain[len(chain)-1]
		tctx.CallGraph.Record(parent, spec.Name)
	}
	chain = append(chain, spec.Name)
	execCtx := withCallChain(ctx, chain)
	execCtx, cancel := withToolTimeout(execCtx, tctx.Config, spec)
	result, toolErr := spec.Handler(execCtx, ToolRequest{Arguments: args, User: user, Context: tctx})
	cancel()
	outcome := "success"
	if toolErr != nil {
		outcome = "error"
		result.Data = canonicalErrorPayload(toolErr, result.Data)
	}
	// Redact tokens/secrets from the result before it leaves the server.
	// This covers k8s.logs payloads, observability.logs.* entries, and any
	// other handler that surfaces strings sourced from user workloads.
	if tctx.Redactor != nil {
		result.Data = tctx.Redactor.RedactValue(result.Data)
	}
	cache := i.skillCache.Load()
	guidance, guidanceErr := customSkillGuidanceForTool(tctx.Config, spec, args, cache)
	result = attachCustomSkillGuidance(result, guidance, guidanceErr)
	logAudit(execCtx, tctx, spec, user.ID, result.Metadata.Namespaces, result.Metadata.Resources, outcome, toolErr)
	return result, toolErr
}

func maxCallDepth(cfg *config.Config) int {
	if cfg == nil {
		return defaultMaxCallDepth
	}
	if cfg.Limits.MaxCallDepth > 0 {
		return cfg.Limits.MaxCallDepth
	}
	return defaultMaxCallDepth
}

const defaultMaxCallDepth = 8

func canonicalErrorPayload(err error, details any) map[string]any {
	if IsErrorEnvelope(details) {
		return details.(map[string]any)
	}
	return BuildErrorEnvelope(err, details)
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

func (i *ToolInvoker) runMutationPreflight(ctx context.Context, rt *invokerRuntime, user policy.User, args map[string]any, pre *PreflightSpec) error {
	if i == nil || rt == nil || rt.reg == nil || pre == nil || pre.GuardTool == "" {
		return nil
	}
	preflight, ok := rt.reg.Get(pre.GuardTool)
	if !ok {
		return fmt.Errorf("preflight guard %s tool is required for mutating operations", pre.GuardTool)
	}
	preflightArgs := map[string]any{}
	maps.Copy(preflightArgs, args)
	if pre.Operation != "" {
		preflightArgs["operation"] = pre.Operation
	}
	result, err := preflight.Handler(ctx, ToolRequest{Arguments: preflightArgs, User: user, Context: rt.ctx})
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
			return fmt.Errorf("mutation preflight failed for operation %s: %v", pre.Operation, summary)
		}
		return fmt.Errorf("mutation preflight failed for operation %s", pre.Operation)
	}
	return nil
}
