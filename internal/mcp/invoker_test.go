package mcp

import (
	"context"
	"errors"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/policy"
)

func TestInvokerToolNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	invoker := NewToolInvoker(reg, ToolContext{})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "missing", nil)
	if err == nil {
		t.Fatalf("expected error for missing tool")
	}
	root, ok := result.Data.(map[string]any)
	if !ok || root["error"] == nil {
		t.Fatalf("expected error envelope in result data: %#v", result.Data)
	}
}

func TestInvokerHandlerError(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"error": "fail"}}, errors.New("fail")
		},
	})
	ctx := ToolContext{Policy: policy.NewAuthorizer()}
	invoker := NewToolInvoker(reg, ctx)
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected handler error")
	}
}

func TestInvokerMissingRegistry(t *testing.T) {
	invoker := &ToolInvoker{}
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err == nil {
		t.Fatalf("expected error for missing registry")
	}
	root, ok := result.Data.(map[string]any)
	if !ok || root["error"] == nil {
		t.Fatalf("expected error envelope in result data: %#v", result.Data)
	}
}

func TestInvokerSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer()})
	result, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "demo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data == nil {
		t.Fatalf("expected result data")
	}
}

func TestInvokerRunsMutationPreflight(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.safe_mutation_preflight",
		ToolsetID: "k8s",
		Safety:    SafetyReadOnly,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			if op, _ := req.Arguments["operation"].(string); op != "apply" {
				t.Fatalf("expected apply operation, got %q", op)
			}
			return ToolResult{Data: map[string]any{"safeToProceed": false}}, nil
		},
	})
	_ = reg.Add(ToolSpec{
		Name:      "k8s.apply",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			t.Fatalf("mutation handler should not run when preflight fails")
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})

	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.apply", map[string]any{"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"})
	if err == nil {
		t.Fatalf("expected preflight failure")
	}
}

func TestInvokerFailsWhenPreflightToolMissing(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.apply",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.apply", map[string]any{"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"})
	if err == nil {
		t.Fatalf("expected error when preflight tool is missing")
	}
}

func TestInvokerFailsWhenPreflightResponseMalformed(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.safe_mutation_preflight",
		ToolsetID: "k8s",
		Safety:    SafetyReadOnly,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"checks": []any{}}}, nil
		},
	})
	_ = reg.Add(ToolSpec{
		Name:      "k8s.patch",
		ToolsetID: "k8s",
		Safety:    SafetyRiskyWrite,
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleCluster}, "k8s.patch", map[string]any{"name": "x", "patch": "{}"})
	if err == nil {
		t.Fatalf("expected malformed preflight response error")
	}
}

func TestInvokerChecksNamespaceForNamespaceRole(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "k8s.namespaced_demo",
		ToolsetID: "k8s",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"namespace": map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"team-a"}}, "k8s.namespaced_demo", map[string]any{"namespace": "team-b"})
	if err == nil {
		t.Fatalf("expected namespace policy error")
	}
}

func TestInvokerDeniesClusterScopedForNamespaceRole(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	_ = reg.Add(ToolSpec{
		Name:      "cluster.info",
		ToolsetID: "cluster",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	invoker := NewToolInvoker(reg, ToolContext{Policy: policy.NewAuthorizer(), Config: &cfg})
	_, err := invoker.Call(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"team-a"}}, "cluster.info", map[string]any{})
	if err == nil {
		t.Fatalf("expected cluster-scoped policy error")
	}
}
