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
