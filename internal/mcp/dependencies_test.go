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
	reg := NewRegistry(nil)
	_ = reg.Add(ToolSpec{Name: "rootcause.incident_bundle", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.overview", Safety: SafetyReadOnly})

	err := ValidateToolDependencies(reg, RequiredToolDependencies())
	if err == nil {
		t.Fatalf("expected dependency validation error")
	}
}
