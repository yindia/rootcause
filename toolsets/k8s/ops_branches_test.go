package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleGetDescribeDeleteClusterScoped(t *testing.T) {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName("demo")
	toolset := newClusterScopeToolset(ns)
	toolset.ctx.Evidence = stubCollector{}

	if _, err := toolset.handleGet(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Namespace", "name": "demo"},
	}); err != nil {
		t.Fatalf("handleGet cluster-scoped: %v", err)
	}

	if _, err := toolset.handleDescribe(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Namespace", "name": "demo"},
	}); err != nil {
		t.Fatalf("handleDescribe cluster-scoped: %v", err)
	}

	if _, err := toolset.handleDelete(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Namespace", "name": "demo", "confirm": true},
	}); err != nil {
		t.Fatalf("handleDelete cluster-scoped: %v", err)
	}
}

func TestHandleEventsAllNamespaces(t *testing.T) {
	event := &corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "default"}}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	client := fake.NewSimpleClientset(event, ns)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	if _, err := toolset.handleEvents(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err != nil {
		t.Fatalf("handleEvents all namespaces: %v", err)
	}
}

func TestHandleAPIResourcesLimitAndMatch(t *testing.T) {
	discoveryClient := &apiDiscovery{
		resourcesByGV: map[string]*metav1.APIResourceList{
			"v1": {
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{{
					Name:        "pods",
					Kind:        "Pod",
					ShortNames:  []string{"po"},
					Categories:  []string{"all"},
					SingularName: "pod",
				}},
			},
		},
	}
	toolset := New()
	cfg := config.DefaultConfig()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: discoveryClient},
		Policy:  policy.NewAuthorizer(),
	})
	if _, err := toolset.handleAPIResources(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"query": "po", "limit": float64(1)},
	}); err != nil {
		t.Fatalf("handleAPIResources limit: %v", err)
	}
	if !apiResourceMatches("all", "v1", metav1.APIResource{Name: "pods", Kind: "Pod", Categories: []string{"all"}}) {
		t.Fatalf("expected apiResourceMatches category")
	}
}

func TestHandleExecReadonlyCommandNotAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Exec.AllowedCommands = []string{"ls"}
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolset.handleExecReadonly(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "api",
			"command":   []any{"bash"},
		},
	}); err == nil {
		t.Fatalf("expected command not allowed error")
	}
}
