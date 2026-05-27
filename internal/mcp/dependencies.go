package mcp

import (
	"fmt"
	"sort"
	"strings"
)

type ToolDependency struct {
	Tool     string
	Requires []string
	// Optional declares soft dependencies — tools that are invoked when present
	// but skipped silently when absent. Shown in the capabilities graph as
	// `source: "optional"` edges; never trigger validation errors.
	Optional []string
}

func RequiredToolDependencies() []ToolDependency {
	deps := []ToolDependency{
		{
			// k8s.* steps are Optional, not Required: the bundle chain calls
			// them when present and records a per-step error when absent, so
			// enabling `rootcause` without `k8s` degrades gracefully instead
			// of aborting server startup.
			Tool:     "rootcause.incident_bundle",
			Optional: []string{"k8s.overview", "k8s.events_timeline", "k8s.diagnose", "gcp.metrics.workload", "gcp.logs.workload"},
		},
		{Tool: "rootcause.change_timeline", Optional: []string{"k8s.events_timeline"}},
		// Intra-k8s dependencies stay Required: if k8s.diagnose is registered,
		// its debug_flow/graph backing tools must be too (same toolset).
		{Tool: "k8s.diagnose", Requires: []string{"k8s.debug_flow"}},
		{Tool: "k8s.debug_flow", Requires: []string{"k8s.graph"}},
		{
			Tool:     "gcp.logs.correlated_with_bundle",
			Optional: []string{"rootcause.incident_bundle"},
		},
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
