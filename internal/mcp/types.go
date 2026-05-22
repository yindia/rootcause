package mcp

import (
	"context"

	"github.com/xeipuuv/gojsonschema"

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
	Name             string
	Description      string
	ToolsetID        string
	InputSchema      map[string]any
	Safety           ToolSafety
	Handler          ToolHandler
	Preflight        *PreflightSpec
	augmentedCache   map[string]any
	compiledSchema   *gojsonschema.Schema
	schemaCompileErr error
}

type PreflightSpec struct {
	GuardTool string
	Operation string
}

func (s *ToolSpec) CompileSchema() (*gojsonschema.Schema, error) {
	if s == nil {
		return nil, nil
	}
	if s.compiledSchema != nil || s.schemaCompileErr != nil {
		return s.compiledSchema, s.schemaCompileErr
	}
	if s.InputSchema == nil {
		return nil, nil
	}
	loader := gojsonschema.NewGoLoader(s.InputSchema)
	compiled, err := gojsonschema.NewSchema(loader)
	if err != nil {
		s.schemaCompileErr = err
		return nil, err
	}
	s.compiledSchema = compiled
	return compiled, nil
}

func (s *ToolSpec) AugmentedSchema() map[string]any {
	if s == nil {
		return nil
	}
	if s.augmentedCache != nil {
		return s.augmentedCache
	}
	schema := s.InputSchema
	if schema == nil {
		schema = map[string]any{"type": "object"}
	}
	s.augmentedCache = schemaWithGlobalSkillTags(schema)
	return s.augmentedCache
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
	Namespaces       []string        `json:"namespaces,omitempty"`
	Resources        []string        `json:"resources,omitempty"`
	CustomSkills     []SkillGuidance `json:"customSkills,omitempty"`
	CustomSkillError string          `json:"customSkillError,omitempty"`
}

func (m *ToolMetadata) Merge(other ToolMetadata) {
	if m == nil {
		return
	}
	m.Namespaces = mergeUniqueStrings(m.Namespaces, other.Namespaces)
	m.Resources = mergeUniqueStrings(m.Resources, other.Resources)
}

func mergeUniqueStrings(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst)+len(src))
	out := make([]string, 0, len(dst)+len(src))
	for _, v := range dst {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range src {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

type SkillGuidance struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags,omitempty"`
	Content     string   `json:"content"`
	Truncated   bool     `json:"truncated"`
}

type ToolContext struct {
	Config    *config.Config
	Clients   *kube.Clients
	Policy    *policy.Authorizer
	Evidence  evidence.Collector
	Renderer  render.Renderer
	Redactor  *redact.Redactor
	Audit     *audit.Logger
	Cache     *cache.Store
	CallGraph *CallGraph
	Invoker   *ToolInvoker
	Registry  Registry
}


