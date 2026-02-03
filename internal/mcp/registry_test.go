package mcp

import (
	"testing"

	"rootcause/internal/config"
)

func TestRegistryReadOnlyFiltersWriteTools(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReadOnly = true
	reg := NewRegistry(&cfg)

	_ = reg.Add(ToolSpec{Name: "k8s.get", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "k8s.apply", Safety: SafetyRiskyWrite})

	if _, ok := reg.Get("k8s.get"); !ok {
		t.Fatalf("expected k8s.get to be registered")
	}
	if _, ok := reg.Get("k8s.apply"); ok {
		t.Fatalf("expected k8s.apply to be filtered in read-only mode")
	}
}

func TestRegistryDisableDestructiveFilters(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DisableDestructive = true
	cfg.Safety.AllowDestructiveTools = []string{"k8s.delete"}
	reg := NewRegistry(&cfg)

	_ = reg.Add(ToolSpec{Name: "k8s.delete", Safety: SafetyDestructive})
	_ = reg.Add(ToolSpec{Name: "k8s.patch", Safety: SafetyRiskyWrite})
	_ = reg.Add(ToolSpec{Name: "k8s.get", Safety: SafetyReadOnly})

	if _, ok := reg.Get("k8s.delete"); !ok {
		t.Fatalf("expected k8s.delete to be allowlisted")
	}
	if _, ok := reg.Get("k8s.patch"); ok {
		t.Fatalf("expected k8s.patch to be filtered when destructive disabled")
	}
	if _, ok := reg.Get("k8s.get"); !ok {
		t.Fatalf("expected k8s.get to be registered")
	}
}
