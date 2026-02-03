package mcp

import (
	"errors"
	"sort"

	"rootcause/internal/config"
)

type Registry interface {
	Add(spec ToolSpec) error
	List() []ToolInfo
	Get(name string) (ToolSpec, bool)
}

type ToolRegistry struct {
	cfg   *config.Config
	tools map[string]ToolSpec
}

func NewRegistry(cfg *config.Config) *ToolRegistry {
	return &ToolRegistry{cfg: cfg, tools: map[string]ToolSpec{}}
}

func (r *ToolRegistry) Add(spec ToolSpec) error {
	if spec.Name == "" {
		return errors.New("tool name required")
	}
	if !r.allowedBySafety(spec) {
		return nil
	}
	r.tools[spec.Name] = spec
	return nil
}

func (r *ToolRegistry) List() []ToolInfo {
	infos := make([]ToolInfo, 0, len(r.tools))
	for _, tool := range r.tools {
		infos = append(infos, ToolInfo{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

func (r *ToolRegistry) Get(name string) (ToolSpec, bool) {
	spec, ok := r.tools[name]
	return spec, ok
}

func (r *ToolRegistry) Specs() []ToolSpec {
	specs := make([]ToolSpec, 0, len(r.tools))
	for _, tool := range r.tools {
		specs = append(specs, tool)
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

func (r *ToolRegistry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *ToolRegistry) allowedBySafety(spec ToolSpec) bool {
	if r.cfg == nil {
		return true
	}
	if r.cfg.ReadOnly {
		return spec.Safety == SafetyReadOnly
	}
	if r.cfg.DisableDestructive {
		if spec.Safety == SafetyDestructive || spec.Safety == SafetyRiskyWrite {
			for _, allow := range r.cfg.Safety.AllowDestructiveTools {
				if allow == spec.Name {
					return true
				}
			}
			return false
		}
	}
	return true
}
