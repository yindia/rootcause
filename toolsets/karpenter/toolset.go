package karpenter

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
	mcp.MustRegisterToolset("karpenter", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "karpenter"
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
			Name:        "karpenter.status",
			Description: "Check Karpenter control-plane status.",
			ToolsetID:   t.ID(),
			InputSchema: schemaStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleStatus,
		},
		{
			Name:        "karpenter.cr_status",
			Description: "Fetch Karpenter CR status for debugging (best-effort).",
			ToolsetID:   t.ID(),
			InputSchema: schemaCRStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCRStatus,
		},
		{
			Name:        "karpenter.node_provisioning_debug",
			Description: "Diagnose pending pods and Karpenter provisioning.",
			ToolsetID:   t.ID(),
			InputSchema: schemaNodeProvisioningDebug(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleNodeProvisioningDebug,
		},
	}
	for _, tool := range tools {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}
