package browser

import (
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/mcp"
)

func TestRegisterDisabledByEnv(t *testing.T) {
	t.Setenv("MCP_BROWSER_ENABLED", "false")
	ts := New()
	if err := ts.Init(mcp.ToolsetContext{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	reg := mcp.NewRegistry(&config.Config{})
	if err := ts.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(reg.Names()) != 0 {
		t.Fatalf("expected no browser tools when disabled")
	}
}

func TestRegisterEnabledByEnv(t *testing.T) {
	t.Setenv("MCP_BROWSER_ENABLED", "true")
	ts := New()
	if err := ts.Init(mcp.ToolsetContext{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	reg := mcp.NewRegistry(&config.Config{})
	if err := ts.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	names := reg.Names()
	if len(names) != 26 {
		t.Fatalf("expected 26 browser tools, got %d", len(names))
	}
	required := []string{"browser_open", "browser_screenshot", "browser_click", "browser_fill", "browser_health_check", "browser_test_ingress", "browser_screenshot_grafana"}
	for _, name := range required {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("missing required browser tool: %s", name)
		}
	}
}
