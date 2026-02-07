package mcp

import (
	"testing"

	"rootcause/internal/config"
)

func TestRegistrySafetyReadOnly(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ReadOnly = true
	reg := NewRegistry(&cfg)
	if err := reg.Add(ToolSpec{Name: "k8s.delete", Safety: SafetyDestructive}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := reg.Get("k8s.delete"); ok {
		t.Fatalf("expected destructive tool to be filtered in read-only mode")
	}
}

func TestRegistrySafetyAllowlist(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DisableDestructive = true
	cfg.Safety.AllowDestructiveTools = []string{"k8s.delete"}
	reg := NewRegistry(&cfg)
	if err := reg.Add(ToolSpec{Name: "k8s.delete", Safety: SafetyDestructive}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := reg.Get("k8s.delete"); !ok {
		t.Fatalf("expected allowlisted tool to be registered")
	}
}

func TestRegistryAddRequiresName(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	if err := reg.Add(ToolSpec{}); err == nil {
		t.Fatalf("expected error for missing tool name")
	}
}

func TestRegistryListAndNames(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{Name: "a", Safety: SafetyReadOnly})
	_ = reg.Add(ToolSpec{Name: "b", Safety: SafetyReadOnly})
	list := reg.List()
	if len(list) != 2 || list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("unexpected list: %#v", list)
	}
	names := reg.Names()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("unexpected names: %#v", names)
	}
}
