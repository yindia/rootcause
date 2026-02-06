package istio

import (
	"errors"
	"fmt"

	"rootcause/internal/mcp"
)

type Toolset struct {
	ctx mcp.ToolsetContext
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("istio", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "istio"
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
			Name:        "istio.health",
			Description: "Check Istio control-plane health.",
			ToolsetID:   t.ID(),
			InputSchema: schemaHealth(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleHealth,
		},
		{
			Name:        "istio.config_summary",
			Description: "Summarize Istio configuration resources.",
			ToolsetID:   t.ID(),
			InputSchema: schemaConfigSummary(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleConfigSummary,
		},
		{
			Name:        "istio.proxy_status",
			Description: "Check Istio proxy sidecar status.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyStatus,
		},
		{
			Name:        "istio.service_mesh_hosts",
			Description: "List service mesh hosts referenced by Istio resources.",
			ToolsetID:   t.ID(),
			InputSchema: schemaServiceMeshHosts(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleServiceMeshHosts,
		},
		{
			Name:        "istio.discover_namespaces",
			Description: "Discover namespaces with Istio sidecars and injection density.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDiscoverNamespaces(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDiscoverNamespaces,
		},
		{
			Name:        "istio.pods_by_service",
			Description: "List pods backing a Kubernetes service.",
			ToolsetID:   t.ID(),
			InputSchema: schemaPodsByService(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handlePodsByService,
		},
		{
			Name:        "istio.external_dependency_check",
			Description: "Check external dependencies referenced by Istio resources.",
			ToolsetID:   t.ID(),
			InputSchema: schemaExternalDependencyCheck(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleExternalDependencyCheck,
		},
		{
			Name:        "istio.proxy_clusters",
			Description: "Fetch Envoy proxy cluster configuration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyClusters,
		},
		{
			Name:        "istio.proxy_listeners",
			Description: "Fetch Envoy proxy listener configuration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyListeners,
		},
		{
			Name:        "istio.proxy_routes",
			Description: "Fetch Envoy proxy route configuration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyRoutes,
		},
		{
			Name:        "istio.proxy_endpoints",
			Description: "Fetch Envoy proxy endpoint configuration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyEndpoints,
		},
		{
			Name:        "istio.proxy_bootstrap",
			Description: "Fetch Envoy proxy bootstrap configuration.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyBootstrap,
		},
		{
			Name:        "istio.proxy_config_dump",
			Description: "Fetch full Envoy proxy config dump.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyConfig(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyConfigDump,
		},
		{
			Name:        "istio.cr_status",
			Description: "Fetch Istio CR status for debugging (best-effort).",
			ToolsetID:   t.ID(),
			InputSchema: schemaCRStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCRStatus,
		},
		{
			Name:        "istio.virtualservice_status",
			Description: "Fetch VirtualService status for debugging.",
			ToolsetID:   t.ID(),
			InputSchema: schemaVirtualServiceStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleVirtualServiceStatus,
		},
		{
			Name:        "istio.destinationrule_status",
			Description: "Fetch DestinationRule status for debugging.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDestinationRuleStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDestinationRuleStatus,
		},
		{
			Name:        "istio.gateway_status",
			Description: "Fetch Gateway status for debugging.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGatewayStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGatewayStatus,
		},
		{
			Name:        "istio.httproute_status",
			Description: "Fetch HTTPRoute status for debugging.",
			ToolsetID:   t.ID(),
			InputSchema: schemaHTTPRouteStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleHTTPRouteStatus,
		},
	}
	for _, tool := range tools {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}
