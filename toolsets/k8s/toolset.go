package k8s

import (
	"errors"
	"fmt"
	"strings"

	"rootcause/internal/mcp"
)

type Toolset struct {
	ctx mcp.ToolsetContext
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("k8s", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "k8s"
}

func (t *Toolset) Version() string {
	return "0.1.0"
}

func (t *Toolset) Init(ctx mcp.ToolsetContext) error {
	if ctx.Clients == nil {
		return errors.New("missing kube clients")
	}
	t.ctx = ctx
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	tools := []mcp.ToolSpec{
		{
			Name:        "k8s.get",
			Description: "Fetch a single Kubernetes object by kind/resource/name (supports CRDs/CRs).",
			ToolsetID:   t.ID(),
			InputSchema: schemaGet(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGet,
		},
		{
			Name:        "k8s.list",
			Description: "List Kubernetes objects by kind/resource (multi-kind + selectors supported).",
			ToolsetID:   t.ID(),
			InputSchema: schemaList(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleList,
		},
		{
			Name:        "k8s.describe",
			Description: "Describe an object with status, events, owners, and related resources.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDescribe(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDescribe,
		},
		{
			Name:        "k8s.delete",
			Description: "Delete a Kubernetes object (destructive).",
			ToolsetID:   t.ID(),
			InputSchema: schemaDelete(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleDelete,
		},
		{
			Name:        "k8s.apply",
			Description: "Server-side apply a YAML manifest (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaApply(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleApply,
		},
		{
			Name:        "k8s.patch",
			Description: "Patch a Kubernetes object (merge/json/strategic; requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaPatch(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handlePatch,
		},
		{
			Name:        "k8s.logs",
			Description: "Read pod logs for troubleshooting.",
			ToolsetID:   t.ID(),
			InputSchema: schemaLogs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleLogs,
		},
		{
			Name:        "k8s.events",
			Description: "List events in a namespace or for an object.",
			ToolsetID:   t.ID(),
			InputSchema: schemaEvents(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleEvents,
		},
		{
			Name:        "k8s.api_resources",
			Description: "Discover available API resources (including CRDs).",
			ToolsetID:   t.ID(),
			InputSchema: schemaAPIResources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleAPIResources,
		},
		{
			Name:        "k8s.resource_usage",
			Description: "Show pod/node CPU+memory usage from metrics-server (kubectl top).",
			ToolsetID:   t.ID(),
			InputSchema: schemaResourceUsage(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleResourceUsage,
		},
		{
			Name:        "k8s.graph",
			Description: "Build a dependency graph for ingress/service/workload/pod traffic flow.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGraph(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGraph,
		},
		{
			Name:        "k8s.crds",
			Description: "List custom resource definitions (CRDs) installed in the cluster.",
			ToolsetID:   t.ID(),
			InputSchema: schemaCRDs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCRDs,
		},
		{
			Name:        "k8s.create",
			Description: "Create resources from YAML manifest (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaCreate(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleCreate,
		},
		{
			Name:        "k8s.scale",
			Description: "Scale a workload by setting replicas (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaScale(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleScale,
		},
		{
			Name:        "k8s.rollout",
			Description: "Check rollout status or restart a deployment (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaRollout(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleRollout,
		},
		{
			Name:        "k8s.context",
			Description: "List, get current, or switch kubeconfig contexts.",
			ToolsetID:   t.ID(),
			InputSchema: schemaContext(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleContext,
		},
		{
			Name:        "k8s.explain_resource",
			Description: "Explain the API schema for a kind/resource (best-effort).",
			ToolsetID:   t.ID(),
			InputSchema: schemaExplain(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleExplain,
		},
		{
			Name:        "k8s.generic",
			Description: "Dispatch a kubectl-style verb when you only know the verb.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGeneric(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGeneric,
		},
		{
			Name:        "k8s.ping",
			Description: "Verify API server connectivity and version.",
			ToolsetID:   t.ID(),
			InputSchema: schemaPing(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handlePing,
		},
		{
			Name:        "k8s.port_forward",
			Description: "Port-forward to a pod or service for a limited duration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaPortForward(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handlePortForward,
		},
		{
			Name:        "k8s.exec",
			Description: "Execute a command in a pod container (no shells unless allowShell).",
			ToolsetID:   t.ID(),
			InputSchema: schemaExec(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleExec,
		},
		{
			Name:        "k8s.cleanup_pods",
			Description: "Delete pods in bad states (evicted, crashloop, image pull backoff).",
			ToolsetID:   t.ID(),
			InputSchema: schemaCleanupPods(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleCleanupPods,
		},
		{
			Name:        "k8s.node_management",
			Description: "Cordon, drain, or uncordon nodes for maintenance.",
			ToolsetID:   t.ID(),
			InputSchema: schemaNodeManagement(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleNodeManagement,
		},
		{
			Name:        "k8s.diagnose",
			Description: "Keyword-driven pod troubleshooting with evidence.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDiagnose(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDiagnose,
		},
		{
			Name:        "k8s.overview",
			Description: "High-level overview of resources in a namespace or cluster.",
			ToolsetID:   t.ID(),
			InputSchema: schemaOverview(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleOverview,
		},
		{
			Name:        "k8s.crashloop_debug",
			Description: "Analyze CrashLoopBackOff pods with events and likely causes.",
			ToolsetID:   t.ID(),
			InputSchema: schemaCrashloopDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCrashloopDebug,
		},
		{
			Name:        "k8s.scheduling_debug",
			Description: "Analyze Pending pods, quotas, priorities, and scheduling blockers.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSchedulingDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSchedulingDebug,
		},
		{
			Name:        "k8s.hpa_debug",
			Description: "Analyze HPA conditions and replica decisions.",
			ToolsetID:   t.ID(),
			InputSchema: schemaHPADebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleHPADebug,
		},
		{
			Name:        "k8s.vpa_debug",
			Description: "Analyze VPA recommendations and target workload metrics.",
			ToolsetID:   t.ID(),
			InputSchema: schemaVPADebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleVPADebug,
		},
		{
			Name:        "k8s.storage_debug",
			Description: "Analyze PVC binding, PV matching, and VolumeAttachment errors.",
			ToolsetID:   t.ID(),
			InputSchema: schemaStorageDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleStorageDebug,
		},
		{
			Name:        "k8s.config_debug",
			Description: "Check ConfigMap/Secret references and missing keys.",
			ToolsetID:   t.ID(),
			InputSchema: schemaConfigDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleConfigDebug,
		},
		{
			Name:        "k8s.debug_flow",
			Description: "Run a graph-driven debug flow (traffic/pending/crashloop/autoscaling/networkpolicy/mesh).",
			ToolsetID:   t.ID(),
			InputSchema: schemaDebugFlow(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDebugFlow,
		},
		{
			Name:        "k8s.network_debug",
			Description: "Analyze Service to Pod networking, endpoints, and NetworkPolicy blocks.",
			ToolsetID:   t.ID(),
			InputSchema: schemaNetworkDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleNetworkDebug,
		},
		{
			Name:        "k8s.private_link_debug",
			Description: "Analyze private link connectivity for a service.",
			ToolsetID:   t.ID(),
			InputSchema: schemaPrivateLinkDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handlePrivateLinkDebug,
		},
	}

	if t.ctx.Config.Exec.Enabled {
		allowed := map[string]struct{}{}
		for _, cmd := range t.ctx.Config.Exec.AllowedCommands {
			allowed[strings.ToLower(cmd)] = struct{}{}
		}
		if len(allowed) > 0 {
			tools = append(tools, mcp.ToolSpec{
				Name:        "k8s.exec_readonly",
				Description: "Execute allowlisted commands in a pod container (output redacted).",
				ToolsetID:   t.ID(),
				InputSchema: schemaExecReadonly(),
				Safety:      mcp.SafetyRiskyWrite,
				Handler:     t.handleExecReadonly,
			})
		}
	}

	for _, tool := range tools {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	aliases := []mcp.ToolSpec{
		{Name: "kubectl_get", Description: "Alias of k8s.get (kubectl get).", ToolsetID: t.ID(), InputSchema: schemaGet(), Safety: mcp.SafetyReadOnly, Handler: t.handleGet},
		{Name: "kubectl_list", Description: "Alias of k8s.list (kubectl get -l/-A).", ToolsetID: t.ID(), InputSchema: schemaList(), Safety: mcp.SafetyReadOnly, Handler: t.handleList},
		{Name: "kubectl_describe", Description: "Alias of k8s.describe (kubectl describe).", ToolsetID: t.ID(), InputSchema: schemaDescribe(), Safety: mcp.SafetyReadOnly, Handler: t.handleDescribe},
		{Name: "kubectl_create", Description: "Alias of k8s.create (kubectl create).", ToolsetID: t.ID(), InputSchema: schemaCreate(), Safety: mcp.SafetyWrite, Handler: t.handleCreate},
		{Name: "kubectl_apply", Description: "Alias of k8s.apply (kubectl apply).", ToolsetID: t.ID(), InputSchema: schemaApply(), Safety: mcp.SafetyRiskyWrite, Handler: t.handleApply},
		{Name: "kubectl_delete", Description: "Alias of k8s.delete (kubectl delete).", ToolsetID: t.ID(), InputSchema: schemaDelete(), Safety: mcp.SafetyDestructive, Handler: t.handleDelete},
		{Name: "kubectl_logs", Description: "Alias of k8s.logs (kubectl logs).", ToolsetID: t.ID(), InputSchema: schemaLogs(), Safety: mcp.SafetyReadOnly, Handler: t.handleLogs},
		{Name: "kubectl_patch", Description: "Alias of k8s.patch (kubectl patch).", ToolsetID: t.ID(), InputSchema: schemaPatch(), Safety: mcp.SafetyRiskyWrite, Handler: t.handlePatch},
		{Name: "kubectl_scale", Description: "Alias of k8s.scale (kubectl scale).", ToolsetID: t.ID(), InputSchema: schemaScale(), Safety: mcp.SafetyWrite, Handler: t.handleScale},
		{Name: "kubectl_rollout", Description: "Alias of k8s.rollout (kubectl rollout).", ToolsetID: t.ID(), InputSchema: schemaRollout(), Safety: mcp.SafetyWrite, Handler: t.handleRollout},
		{Name: "kubectl_context", Description: "Alias of k8s.context (kubectl config).", ToolsetID: t.ID(), InputSchema: schemaContext(), Safety: mcp.SafetyReadOnly, Handler: t.handleContext},
		{Name: "kubectl_generic", Description: "Alias of k8s.generic for generic kubectl verbs.", ToolsetID: t.ID(), InputSchema: schemaGeneric(), Safety: mcp.SafetyReadOnly, Handler: t.handleGeneric},
		{Name: "explain_resource", Description: "Alias of k8s.explain_resource.", ToolsetID: t.ID(), InputSchema: schemaExplain(), Safety: mcp.SafetyReadOnly, Handler: t.handleExplain},
		{Name: "list_api_resources", Description: "Alias of k8s.api_resources.", ToolsetID: t.ID(), InputSchema: schemaAPIResources(), Safety: mcp.SafetyReadOnly, Handler: t.handleAPIResources},
		{Name: "kubectl_top", Description: "Alias of k8s.resource_usage (kubectl top).", ToolsetID: t.ID(), InputSchema: schemaResourceUsage(), Safety: mcp.SafetyReadOnly, Handler: t.handleResourceUsage},
		{Name: "ping", Description: "Alias of k8s.ping.", ToolsetID: t.ID(), InputSchema: schemaPing(), Safety: mcp.SafetyReadOnly, Handler: t.handlePing},
		{Name: "port_forward", Description: "Alias of k8s.port_forward.", ToolsetID: t.ID(), InputSchema: schemaPortForward(), Safety: mcp.SafetyReadOnly, Handler: t.handlePortForward},
	}
	for _, tool := range aliases {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}

func (t *Toolset) toolContext() mcp.ToolContext {
	return mcp.ToolContext(t.ctx)
}

func (t *Toolset) checkAllowedNamespace(userNamespaces []string, namespace string) error {
	if namespace == "" {
		return nil
	}
	for _, allowed := range userNamespaces {
		if allowed == namespace {
			return nil
		}
	}
	return fmt.Errorf("namespace %s not allowed", namespace)
}

func resourceRef(kind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", kind, name)
	}
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}
}

func (t *Toolset) requireArgs(args map[string]any, keys ...string) error {
	for _, key := range keys {
		if _, ok := args[key]; !ok {
			return fmt.Errorf("missing required field: %s", key)
		}
	}
	return nil
}

func requireConfirm(args map[string]any) error {
	if val, ok := args["confirm"].(bool); ok && val {
		return nil
	}
	return errors.New("confirmation required: set confirm=true to proceed")
}

func toString(val any) string {
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

func toStringSlice(val any) []string {
	if val == nil {
		return nil
	}
	items, ok := val.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
