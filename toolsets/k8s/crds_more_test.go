package k8s

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestHandleCRDsQueryAndLimit(t *testing.T) {
	crd1 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "widgets.example.com"},
		"spec":       map[string]any{"group": "example.com", "scope": "Namespaced", "names": map[string]any{"kind": "Widget", "plural": "widgets"}},
	}}
	crd2 := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "gadgets.example.com"},
		"spec":       map[string]any{"group": "example.com", "scope": "Namespaced", "names": map[string]any{"kind": "Gadget", "plural": "gadgets"}},
	}}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		crdGVR: "CustomResourceDefinitionList",
	}, crd1, crd2)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dynamicClient},
		Policy:  policy.NewAuthorizer(),
	})

	if _, err := toolset.handleCRDs(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"query": "widget", "limit": float64(1)},
	}); err != nil {
		t.Fatalf("handleCRDs query/limit: %v", err)
	}
}
