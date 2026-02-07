package karpenter

import (
	"errors"
	"testing"

	"rootcause/internal/mcp"
)

type errorRegistry struct{}

func (errorRegistry) Add(mcp.ToolSpec) error            { return errors.New("boom") }
func (errorRegistry) List() []mcp.ToolInfo              { return nil }
func (errorRegistry) Get(string) (mcp.ToolSpec, bool)   { return mcp.ToolSpec{}, false }

func TestRegisterError(t *testing.T) {
	toolset := New()
	if err := toolset.Register(errorRegistry{}); err == nil {
		t.Fatalf("expected register error")
	}
}
