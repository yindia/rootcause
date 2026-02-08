package mcp

import (
	"context"

	"rootcause/internal/audit"
	"rootcause/internal/cache"
	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type ToolSafety string

const (
	SafetyReadOnly    ToolSafety = "read_only"
	SafetyWrite       ToolSafety = "write"
	SafetyRiskyWrite  ToolSafety = "risky_write"
	SafetyDestructive ToolSafety = "destructive"
)

type ToolHandler func(ctx context.Context, req ToolRequest) (ToolResult, error)

type ToolSpec struct {
	Name        string
	Description string
	ToolsetID   string
	InputSchema map[string]any
	Safety      ToolSafety
	Handler     ToolHandler
}

type ToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type ToolRequest struct {
	Arguments map[string]any
	User      policy.User
	Context   ToolContext
}

type ToolResult struct {
	Data     any
	Metadata ToolMetadata
}

type ToolMetadata struct {
	Namespaces []string `json:"namespaces,omitempty"`
	Resources  []string `json:"resources,omitempty"`
}

type ToolContext struct {
	Config   *config.Config
	Clients  *kube.Clients
	Policy   *policy.Authorizer
	Evidence evidence.Collector
	Renderer render.Renderer
	Redactor *redact.Redactor
	Audit    *audit.Logger
	Services *ServiceRegistry
	Cache    *cache.Store
	Invoker  *ToolInvoker
	Registry Registry
}

type ToolsetContext = ToolContext
