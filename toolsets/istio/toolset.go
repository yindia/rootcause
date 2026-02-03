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
			Name:        "istio.proxy_status",
			Description: "Check Istio proxy sidecar status.",
			ToolsetID:   t.ID(),
			InputSchema: schemaProxyStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleProxyStatus,
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
