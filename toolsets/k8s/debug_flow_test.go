package k8s

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newDebugFlowToolset(graph map[string]any) *Toolset {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Registry: reg,
	}
	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	toolset := New()
	_ = toolset.Init(ctx)

	_ = reg.Add(mcp.ToolSpec{
		Name:      "k8s.graph",
		ToolsetID: "k8s",
		Handler: func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: graph}, nil
		},
	})
	stub := func(name string) mcp.ToolSpec {
		return mcp.ToolSpec{
			Name:      name,
			ToolsetID: "k8s",
			Handler: func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error) {
				return mcp.ToolResult{Data: map[string]any{"tool": name}}, nil
			},
		}
	}
	for _, name := range []string{
		"k8s.describe",
		"k8s.network_debug",
		"k8s.scheduling_debug",
		"k8s.crashloop_debug",
		"k8s.storage_debug",
		"k8s.config_debug",
		"k8s.hpa_debug",
		"k8s.vpa_debug",
		"k8s.resource_usage",
		"k8s.permission_debug",
		"istio.service_mesh_hosts",
		"linkerd.policy_debug",
	} {
		_ = reg.Add(stub(name))
	}

	return toolset
}

func baseFlowGraph() map[string]any {
	return map[string]any{
		"nodes": []any{
			map[string]any{"id": "service/default/api", "kind": "Service", "name": "api", "namespace": "default"},
			map[string]any{"id": "pod/default/api-1", "kind": "Pod", "name": "api-1", "namespace": "default"},
			map[string]any{"id": "deployment/default/api", "kind": "Deployment", "name": "api", "namespace": "default"},
			map[string]any{"id": "networkpolicy/default/deny", "kind": "NetworkPolicy", "name": "deny", "namespace": "default"},
			map[string]any{"id": "virtualservice.networking.istio.io/default/api", "kind": "VirtualService", "name": "api", "namespace": "default"},
			map[string]any{"id": "serviceprofile.linkerd.io/default/api", "kind": "ServiceProfile", "name": "api", "namespace": "default"},
			map[string]any{"id": "serviceaccount/default/default", "kind": "ServiceAccount", "name": "default", "namespace": "default"},
		},
		"edges": []any{
			map[string]any{"from": "service/default/api", "to": "pod/default/api-1", "relation": "routes-to"},
			map[string]any{"from": "pod/default/api-1", "to": "deployment/default/api", "relation": "owned-by"},
			map[string]any{"from": "networkpolicy/default/deny", "to": "pod/default/api-1", "relation": "selects"},
			map[string]any{"from": "pod/default/api-1", "to": "networkpolicy/default/deny", "relation": "blocked-by"},
		},
	}
}

func TestParseGraphHelpers(t *testing.T) {
	graph, warnings, err := parseGraph(baseFlowGraph())
	if err != nil {
		t.Fatalf("parseGraph: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	services := relatedByKind(graph, []string{"service/default/api"}, "Service")
	if len(services) != 0 {
		t.Fatalf("expected no related services from service, got %d", len(services))
	}
	pods := relatedPodsForServices(graph, []flowNode{{ID: "service/default/api", Kind: "Service", Name: "api", Namespace: "default"}})
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	owners := relatedWorkloadsForPods(graph, pods)
	if len(owners) != 1 {
		t.Fatalf("expected 1 owner, got %d", len(owners))
	}
	policies := policiesForPods(graph, pods)
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if !hasKindGroup(graph, "VirtualService", "networking.istio.io") {
		t.Fatalf("expected istio group to be detected")
	}
	if len(podsFromEntry(graph, "service/default/api")) == 0 {
		t.Fatalf("expected pods from entry")
	}
}

func TestHandleDebugFlowScenarios(t *testing.T) {
	toolset := newDebugFlowToolset(baseFlowGraph())
	user := policy.User{Role: policy.RoleCluster}
	scenarios := []string{"traffic", "pending", "crashloop", "autoscaling", "networkpolicy", "mesh", "permission"}
	for _, scenario := range scenarios {
		kind := "service"
		name := "api"
		if scenario == "permission" {
			kind = "pod"
			name = "api-1"
		}
		result, err := toolset.handleDebugFlow(context.Background(), mcp.ToolRequest{
			User: user,
			Arguments: map[string]any{
				"namespace": "default",
				"kind":      kind,
				"name":      name,
				"scenario":  scenario,
				"maxSteps":  10,
			},
		})
		if err != nil {
			t.Fatalf("handleDebugFlow %s: %v", scenario, err)
		}
		data, ok := result.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map result for %s", scenario)
		}
		switch steps := data["steps"].(type) {
		case []flowStep:
			if len(steps) == 0 {
				t.Fatalf("expected steps for %s", scenario)
			}
		case []any:
			if len(steps) == 0 {
				t.Fatalf("expected steps for %s", scenario)
			}
		default:
			t.Fatalf("unexpected steps type for %s", scenario)
		}
	}
}

func TestHandleDebugFlowInvalid(t *testing.T) {
	toolset := newDebugFlowToolset(baseFlowGraph())
	_, err := toolset.handleDebugFlow(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err == nil {
		t.Fatalf("expected error for missing args")
	}
}
