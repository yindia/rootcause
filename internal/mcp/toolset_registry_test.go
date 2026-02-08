package mcp

import "testing"

func resetToolsetRegistry() {
	registry = toolsetRegistry{factories: map[string]ToolsetFactory{}}
}

func TestRegisterToolsetErrors(t *testing.T) {
	resetToolsetRegistry()
	if err := RegisterToolset("", func() Toolset { return nil }); err == nil {
		t.Fatalf("expected error for empty id")
	}
	if err := RegisterToolset("demo", nil); err == nil {
		t.Fatalf("expected error for nil factory")
	}
	if err := RegisterToolset("demo", func() Toolset { return nil }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RegisterToolset("demo", func() Toolset { return nil }); err == nil {
		t.Fatalf("expected error for duplicate registration")
	}
}

func TestMustRegisterToolsetPanics(t *testing.T) {
	resetToolsetRegistry()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic from MustRegisterToolset")
		}
	}()
	MustRegisterToolset("", func() Toolset { return nil })
}

func TestToolsetFactoryForAndRegisteredToolsets(t *testing.T) {
	resetToolsetRegistry()
	RegisterToolset("b", func() Toolset { return nil })
	RegisterToolset("a", func() Toolset { return nil })
	if _, ok := ToolsetFactoryFor("missing"); ok {
		t.Fatalf("expected missing toolset")
	}
	if _, ok := ToolsetFactoryFor("a"); !ok {
		t.Fatalf("expected toolset factory")
	}
	ids := RegisteredToolsets()
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("unexpected toolset ids: %#v", ids)
	}
}
