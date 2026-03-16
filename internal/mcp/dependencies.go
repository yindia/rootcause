package mcp

import (
	"fmt"
	"sort"
	"strings"
)

type ToolDependency struct {
	Tool     string
	Requires []string
}

func RequiredToolDependencies() []ToolDependency {
	deps := []ToolDependency{
		{Tool: "rootcause.incident_bundle", Requires: []string{"k8s.overview", "k8s.events_timeline", "k8s.diagnose"}},
		{Tool: "rootcause.change_timeline", Requires: []string{"k8s.events_timeline"}},
		{Tool: "k8s.diagnose", Requires: []string{"k8s.debug_flow"}},
		{Tool: "k8s.debug_flow", Requires: []string{"k8s.graph"}},
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Tool < deps[j].Tool })
	return deps
}

func ValidateToolDependencies(reg Registry, deps []ToolDependency) error {
	if reg == nil {
		return fmt.Errorf("tool registry is required")
	}
	var missing []string
	for _, dep := range deps {
		if _, ok := reg.Get(dep.Tool); !ok {
			continue
		}
		for _, required := range dep.Requires {
			if _, ok := reg.Get(required); !ok {
				missing = append(missing, fmt.Sprintf("%s -> %s", dep.Tool, required))
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("missing required inter-tool dependencies: %s", strings.Join(missing, ", "))
}
