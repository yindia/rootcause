package terraform

import (
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
	mcp.MustRegisterToolset("terraform", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "terraform"
}

func (t *Toolset) Version() string {
	return "0.1.0"
}

func (t *Toolset) Init(ctx mcp.ToolsetContext) error {
	t.ctx = ctx
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	tools := []mcp.ToolSpec{
		{
			Name:        "terraform.debug_plan",
			Description: "Analyze a Terraform plan JSON for risky changes and likely issues.",
			ToolsetID:   t.ID(),
			InputSchema: schemaDebugPlan(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleDebugPlan,
		},
		{
			Name:        "terraform.list_modules",
			Description: "List Terraform modules from the registry.",
			ToolsetID:   t.ID(),
			InputSchema: schemaListModules(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleListModules,
		},
		{
			Name:        "terraform.get_module",
			Description: "Get Terraform module details and versions.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGetModule(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGetModule,
		},
		{
			Name:        "terraform.search_modules",
			Description: "Search Terraform modules by query.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSearchModules(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSearchModules,
		},
		{
			Name:        "terraform.list_providers",
			Description: "List Terraform providers from the registry.",
			ToolsetID:   t.ID(),
			InputSchema: schemaListProviders(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleListProviders,
		},
		{
			Name:        "terraform.get_provider",
			Description: "Get Terraform provider details, versions, and schema metadata.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGetProvider(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGetProvider,
		},
		{
			Name:        "terraform.search_providers",
			Description: "Search Terraform providers by query.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSearchProviders(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSearchProviders,
		},
		{
			Name:        "terraform.list_resources",
			Description: "List Terraform resources for a provider schema.",
			ToolsetID:   t.ID(),
			InputSchema: schemaListResources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleListResources,
		},
		{
			Name:        "terraform.get_resource",
			Description: "Get Terraform resource schema details.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGetResource(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGetResource,
		},
		{
			Name:        "terraform.search_resources",
			Description: "Search Terraform resource schemas by query.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSearchResources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSearchResources,
		},
		{
			Name:        "terraform.list_data_sources",
			Description: "List Terraform data sources for a provider schema.",
			ToolsetID:   t.ID(),
			InputSchema: schemaListDataSources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleListDataSources,
		},
		{
			Name:        "terraform.get_data_source",
			Description: "Get Terraform data source schema details.",
			ToolsetID:   t.ID(),
			InputSchema: schemaGetDataSource(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleGetDataSource,
		},
		{
			Name:        "terraform.search_data_sources",
			Description: "Search Terraform data source schemas by query.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSearchDataSources(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSearchDataSources,
		},
	}

	for _, tool := range tools {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}
