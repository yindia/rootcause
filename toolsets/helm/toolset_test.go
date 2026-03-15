package helm

import (
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestToolsetInitAndRegister(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected error for missing clients")
	}
	ctx := mcp.ToolsetContext{Clients: &kube.Clients{}, Config: &config.Config{}, Policy: policy.NewAuthorizer()}
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("helm.list"); !ok {
		t.Fatalf("expected helm.list to be registered")
	}
	for _, name := range []string{"helm.list_charts", "helm.get_chart", "helm.search_charts", "helm.diff_release", "helm.rollback_advisor"} {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("expected %s to be registered", name)
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	if err := requireConfirm(map[string]any{"confirm": true}); err != nil {
		t.Fatalf("expected confirm to pass: %v", err)
	}
	if err := requireConfirm(map[string]any{}); err == nil {
		t.Fatalf("expected confirm error")
	}
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toString(3); got != "3" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toStringSlice([]any{"a", 1}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toStringSlice("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected toStringSlice string: %#v", got)
	}
	if got := toStringSlice([]string{"a", "b"}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice []string: %#v", got)
	}
	if got := toBool(true); !got {
		t.Fatalf("expected true toBool")
	}
	if got := toInt(float64(4)); got != 4 {
		t.Fatalf("unexpected toInt: %d", got)
	}
}
