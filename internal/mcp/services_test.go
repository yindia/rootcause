package mcp

import "testing"

func TestServiceRegistry(t *testing.T) {
	reg := NewServiceRegistry()
	if err := reg.Register("demo", "value"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("demo"); !ok {
		t.Fatalf("expected service")
	}
	names := reg.Names()
	if len(names) != 1 || names[0] != "demo" {
		t.Fatalf("unexpected names: %#v", names)
	}
	if err := reg.Register("demo", "value2"); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestServiceRegistryErrors(t *testing.T) {
	var reg *ServiceRegistry
	if err := reg.Register("demo", "value"); err == nil {
		t.Fatalf("expected error for nil registry")
	}
	if _, ok := reg.Get("demo"); ok {
		t.Fatalf("expected nil registry get to fail")
	}
	if names := reg.Names(); names != nil {
		t.Fatalf("expected nil names for nil registry")
	}

	reg = NewServiceRegistry()
	if err := reg.Register("", "value"); err == nil {
		t.Fatalf("expected error for empty name")
	}
	if err := reg.Register("demo", nil); err == nil {
		t.Fatalf("expected error for nil service")
	}
}
