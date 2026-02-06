package helm

import (
	"errors"

	"rootcause/internal/mcp"
)

type Toolset struct {
	ctx mcp.ToolsetContext
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("helm", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "helm"
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
			Name:        "helm.repo_add",
			Description: "Add or update a Helm repository (use before install/upgrade).",
			ToolsetID:   t.ID(),
			InputSchema: schemaRepoAdd(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleRepoAdd,
		},
		{
			Name:        "helm.repo_list",
			Description: "List configured Helm repositories.",
			ToolsetID:   t.ID(),
			InputSchema: schemaRepoList(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleRepoList,
		},
		{
			Name:        "helm.repo_update",
			Description: "Update Helm repository indexes.",
			ToolsetID:   t.ID(),
			InputSchema: schemaRepoUpdate(),
			Safety:      mcp.SafetyWrite,
			Handler:     t.handleRepoUpdate,
		},
		{
			Name:        "helm.list",
			Description: "List Helm releases (optionally all namespaces).",
			ToolsetID:   t.ID(),
			InputSchema: schemaList(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleList,
		},
		{
			Name:        "helm.status",
			Description: "Get Helm release status and notes.",
			ToolsetID:   t.ID(),
			InputSchema: schemaStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleStatus,
		},
		{
			Name:        "helm.install",
			Description: "Install a Helm chart (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaInstall(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleInstall,
		},
		{
			Name:        "helm.upgrade",
			Description: "Upgrade a Helm release (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaUpgrade(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleUpgrade,
		},
		{
			Name:        "helm.uninstall",
			Description: "Uninstall a Helm release (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaUninstall(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleUninstall,
		},
		{
			Name:        "helm.template_apply",
			Description: "Render a chart and apply it (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaTemplateApply(),
			Safety:      mcp.SafetyRiskyWrite,
			Handler:     t.handleTemplateApply,
		},
		{
			Name:        "helm.template_uninstall",
			Description: "Render a chart and delete rendered resources (requires confirm=true).",
			ToolsetID:   t.ID(),
			InputSchema: schemaTemplateUninstall(),
			Safety:      mcp.SafetyDestructive,
			Handler:     t.handleTemplateUninstall,
		},
	}
	for _, tool := range tools {
		if err := reg.Add(tool); err != nil {
			return err
		}
	}
	return nil
}
