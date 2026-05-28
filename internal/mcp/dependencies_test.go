package mcp

import "testing"

func TestValidateToolDependenciesSuccess(t *testing.T) {
	reg := NewRegistry(nil)
	_ = reg.Add(ToolSpec{Name: "rootcause.incident_bundle", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.overview", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.events_timeline", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.diagnose", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.debug_flow", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.graph", Safety: SafetyReadOnly})

	if err := ValidateToolDependencies(reg, RequiredToolDependencies()); err != nil {
		t.Fatalf("expected no dependency errors: %v", err)
	}
}

func TestValidateToolDependenciesMissingRequired(t *testing.T) {
	// k8s.diagnose Requires k8s.debug_flow (intra-toolset, still hard). Register
	// diagnose without debug_flow to trigger the required-dependency error.
	reg := NewRegistry(nil)
	_ = reg.Add(ToolSpec{Name: "k8s.diagnose", Safety: SafetyReadOnly})

	err := ValidateToolDependencies(reg, RequiredToolDependencies())
	if err == nil {
		t.Fatalf("expected dependency validation error for k8s.diagnose -> k8s.debug_flow")
	}
}

func TestRootcauseWithoutK8sStartsClean(t *testing.T) {
	// rootcause.incident_bundle's k8s.* deps are Optional, so a registry with
	// rootcause tools but no k8s toolset must validate without error.
	reg := NewRegistry(nil)
	for _, name := range []string{
		"rootcause.incident_bundle",
		"rootcause.change_timeline",
	} {
		_ = reg.Add(ToolSpec{Name: name, Safety: SafetyReadOnly})
	}
	if err := ValidateToolDependencies(reg, RequiredToolDependencies()); err != nil {
		t.Fatalf("expected rootcause to validate without k8s, got: %v", err)
	}
}

func TestValidateToolDependenciesIgnoresMissingOptional(t *testing.T) {
	// All required deps for rootcause.incident_bundle present; gcp.* optional
	// deps are absent. Validation must succeed.
	reg := NewRegistry(nil)
	for _, name := range []string{
		"rootcause.incident_bundle",
		"rootcause.change_timeline",
		"k8s.overview",
		"k8s.events_timeline",
		"k8s.diagnose",
		"k8s.debug_flow",
		"k8s.graph",
	} {
		_ = reg.Add(ToolSpec{Name: name, Safety: SafetyReadOnly})
	}
	if err := ValidateToolDependencies(reg, RequiredToolDependencies()); err != nil {
		t.Fatalf("expected no validation error when only optional deps are missing: %v", err)
	}
}

func TestRequiredToolDependenciesDeclaresGCPOptional(t *testing.T) {
	deps := RequiredToolDependencies()
	var found ToolDependency
	for _, d := range deps {
		if d.Tool == "rootcause.incident_bundle" {
			found = d
			break
		}
	}
	if found.Tool == "" {
		t.Fatalf("expected rootcause.incident_bundle entry")
	}
	if !containsString(found.Optional, "observability.metrics.workload") {
		t.Errorf("expected observability.metrics.workload as optional, got %v", found.Optional)
	}
	if !containsString(found.Optional, "observability.logs.workload") {
		t.Errorf("expected observability.logs.workload as optional, got %v", found.Optional)
	}
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
