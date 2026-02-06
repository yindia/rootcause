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
			Description: "Get a Kubernetes resource by kind or resource name.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGet(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGet,
		},
		{
			Name:        "k8s.list",
			Description: "List Kubernetes resources (multiple kinds supported).",
			ToolsetID:   t.ID(),
			InputSchema: schemaList(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleList,
		},
		{
			Name:        "k8s.describe",
			Description: "Describe a Kubernetes resource with events and related objects.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDescribe(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDescribe,
		},
		{
			Name:        "k8s.delete",
			Description: "Delete a Kubernetes resource.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDelete(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleDelete,
		},
		{
			Name:        "k8s.apply",
			Description: "Server-side apply Kubernetes manifests.",
			ToolsetID:   t.ID(),
			InputSchema: schemaApply(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleApply,
		},
		{
			Name:        "k8s.patch",
			Description: "Patch a Kubernetes resource.",
			ToolsetID:   t.ID(),
			InputSchema: schemaPatch(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handlePatch,
		},
		{
			Name:        "k8s.logs",
			Description: "Fetch pod logs.",
			ToolsetID:   t.ID(),
			InputSchema: schemaLogs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleLogs,
		},
		{
			Name:        "k8s.events",
			Description: "List Kubernetes events.",
			ToolsetID:   t.ID(),
			InputSchema: schemaEvents(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleEvents,
		},
		{
			Name:        "k8s.api_resources",
			Description: "List API resources available in the cluster.",
			ToolsetID:   t.ID(),
			InputSchema: schemaAPIResources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleAPIResources,
		},
		{
			Name:        "k8s.resource_usage",
			Description: "Fetch pod and node resource usage from metrics-server.",
			ToolsetID:   t.ID(),
			InputSchema: schemaResourceUsage(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleResourceUsage,
		},
		{
			Name:        "k8s.graph",
			Description: "Build a topology graph across ingress, service, endpoints, and workloads.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGraph(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGraph,
		},
		{
			Name:        "k8s.crds",
			Description: "List custom resource definitions (CRDs).",
			ToolsetID:   t.ID(),
			InputSchema: schemaCRDs(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCRDs,
		},
		{
			Name:        "k8s.create",
			Description: "Create Kubernetes resources from manifests.",
			ToolsetID:   t.ID(),
			InputSchema: schemaCreate(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleCreate,
		},
		{
			Name:        "k8s.scale",
			Description: "Scale a workload by updating spec.replicas.",
			ToolsetID:   t.ID(),
			InputSchema: schemaScale(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleScale,
		},
		{
			Name:        "k8s.rollout",
			Description: "Get rollout status or restart a deployment.",
			ToolsetID:   t.ID(),
			InputSchema: schemaRollout(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleRollout,
		},
		{
			Name:        "k8s.context",
			Description: "List kubeconfig contexts and current context.",
			ToolsetID:   t.ID(),
			InputSchema: schemaContext(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleContext,
		},
		{
			Name:        "k8s.explain_resource",
			Description: "Explain a Kubernetes resource using discovery metadata (best-effort).",
			ToolsetID:   t.ID(),
			InputSchema: schemaExplain(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleExplain,
		},
		{
			Name:        "k8s.generic",
			Description: "Generic wrapper for kubectl-style verbs (best-effort).",
			ToolsetID:   t.ID(),
			InputSchema: schemaGeneric(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGeneric,
		},
		{
			Name:        "k8s.ping",
			Description: "Verify API server connectivity.",
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
			Description: "Execute a command in a pod container.",
			ToolsetID:   t.ID(),
			InputSchema: schemaExec(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleExec,
		},
		{
			Name:        "k8s.cleanup_pods",
			Description: "Delete pods in problematic states (evicted, crashloop, image pull backoff, etc).",
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
			Description: "Guided troubleshooting for pods based on a keyword.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDiagnose(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDiagnose,
		},
		{
			Name:        "k8s.overview",
			Description: "High-level cluster or namespace overview.",
			ToolsetID:   t.ID(),
			InputSchema: schemaOverview(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleOverview,
		},
		{
			Name:        "k8s.crashloop_debug",
			Description: "Diagnose crash looping pods.",
			ToolsetID:   t.ID(),
			InputSchema: schemaCrashloopDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCrashloopDebug,
		},
		{
			Name:        "k8s.scheduling_debug",
			Description: "Diagnose scheduling failures for pending pods.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSchedulingDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSchedulingDebug,
		},
		{
			Name:        "k8s.hpa_debug",
			Description: "Diagnose HorizontalPodAutoscaler status.",
			ToolsetID:   t.ID(),
			InputSchema: schemaHPADebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleHPADebug,
		},
		{
			Name:        "k8s.vpa_debug",
			Description: "Diagnose VerticalPodAutoscaler recommendations.",
			ToolsetID:   t.ID(),
			InputSchema: schemaVPADebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleVPADebug,
		},
		{
			Name:        "k8s.storage_debug",
			Description: "Diagnose PVC binding and volume attachment issues.",
			ToolsetID:   t.ID(),
			InputSchema: schemaStorageDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleStorageDebug,
		},
		{
			Name:        "k8s.config_debug",
			Description: "Diagnose ConfigMap/Secret references and missing keys.",
			ToolsetID:   t.ID(),
			InputSchema: schemaConfigDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleConfigDebug,
		},
		{
			Name:        "k8s.network_debug",
			Description: "Diagnose service networking issues.",
			ToolsetID:   t.ID(),
			InputSchema: schemaNetworkDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleNetworkDebug,
		},
		{
			Name:        "k8s.private_link_debug",
			Description: "Diagnose private link connectivity for services.",
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
				Description: "Execute allowlisted commands in a pod container.",
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
		{Name: "kubectl_get", Description: "Get a Kubernetes resource.", ToolsetID: t.ID(), InputSchema: schemaGet(), Safety: mcp.SafetyReadOnly, Handler: t.handleGet},
		{Name: "kubectl_list", Description: "List Kubernetes resources.", ToolsetID: t.ID(), InputSchema: schemaList(), Safety: mcp.SafetyReadOnly, Handler: t.handleList},
		{Name: "kubectl_describe", Description: "Describe a Kubernetes resource.", ToolsetID: t.ID(), InputSchema: schemaDescribe(), Safety: mcp.SafetyReadOnly, Handler: t.handleDescribe},
		{Name: "kubectl_create", Description: "Create Kubernetes resources from manifests.", ToolsetID: t.ID(), InputSchema: schemaCreate(), Safety: mcp.SafetyWrite, Handler: t.handleCreate},
		{Name: "kubectl_apply", Description: "Server-side apply manifests.", ToolsetID: t.ID(), InputSchema: schemaApply(), Safety: mcp.SafetyRiskyWrite, Handler: t.handleApply},
		{Name: "kubectl_delete", Description: "Delete a Kubernetes resource.", ToolsetID: t.ID(), InputSchema: schemaDelete(), Safety: mcp.SafetyDestructive, Handler: t.handleDelete},
		{Name: "kubectl_logs", Description: "Fetch pod logs.", ToolsetID: t.ID(), InputSchema: schemaLogs(), Safety: mcp.SafetyReadOnly, Handler: t.handleLogs},
		{Name: "kubectl_patch", Description: "Patch a Kubernetes resource.", ToolsetID: t.ID(), InputSchema: schemaPatch(), Safety: mcp.SafetyRiskyWrite, Handler: t.handlePatch},
		{Name: "kubectl_scale", Description: "Scale a workload by updating spec.replicas.", ToolsetID: t.ID(), InputSchema: schemaScale(), Safety: mcp.SafetyWrite, Handler: t.handleScale},
		{Name: "kubectl_rollout", Description: "Get rollout status or restart a deployment.", ToolsetID: t.ID(), InputSchema: schemaRollout(), Safety: mcp.SafetyWrite, Handler: t.handleRollout},
		{Name: "kubectl_context", Description: "List kubeconfig contexts and current context.", ToolsetID: t.ID(), InputSchema: schemaContext(), Safety: mcp.SafetyReadOnly, Handler: t.handleContext},
		{Name: "kubectl_generic", Description: "Generic wrapper for kubectl-style verbs (best-effort).", ToolsetID: t.ID(), InputSchema: schemaGeneric(), Safety: mcp.SafetyReadOnly, Handler: t.handleGeneric},
		{Name: "explain_resource", Description: "Explain a Kubernetes resource using discovery metadata (best-effort).", ToolsetID: t.ID(), InputSchema: schemaExplain(), Safety: mcp.SafetyReadOnly, Handler: t.handleExplain},
		{Name: "list_api_resources", Description: "List API resources available in the cluster.", ToolsetID: t.ID(), InputSchema: schemaAPIResources(), Safety: mcp.SafetyReadOnly, Handler: t.handleAPIResources},
		{Name: "kubectl_top", Description: "Fetch resource usage from metrics-server.", ToolsetID: t.ID(), InputSchema: schemaResourceUsage(), Safety: mcp.SafetyReadOnly, Handler: t.handleResourceUsage},
		{Name: "ping", Description: "Verify API server connectivity.", ToolsetID: t.ID(), InputSchema: schemaPing(), Safety: mcp.SafetyReadOnly, Handler: t.handlePing},
		{Name: "port_forward", Description: "Port-forward to a pod or service for a limited duration.", ToolsetID: t.ID(), InputSchema: schemaPortForward(), Safety: mcp.SafetyReadOnly, Handler: t.handlePortForward},
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
