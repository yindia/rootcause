package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestAddGroupResourcesClusterScoped(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "sample.io/v1",
		"kind":       "ClusterThing",
		"metadata": map[string]any{
			"name": "alpha",
		},
	}}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme, obj)
	discoveryClient := &apiDiscovery{
		resourcesByGV: map[string]*metav1.APIResourceList{
			"sample.io/v1": {
				GroupVersion: "sample.io/v1",
				APIResources: []metav1.APIResource{
					{Name: "clusterthings", Kind: "ClusterThing", Namespaced: false},
				},
			},
		},
	}

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dynamicClient, Discovery: discoveryClient},
		Policy:  policy.NewAuthorizer(),
	})

	graph := newGraphBuilder()
	cache := newGraphCache()
	warnings := toolset.addGroupResources(context.Background(), graph, "default", "sample.io", map[string]string{}, cache)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if _, ok := graph.nodes[nodeID("ClusterThing", "sample.io", "", "alpha")]; !ok {
		t.Fatalf("expected cluster node in graph")
	}
}
