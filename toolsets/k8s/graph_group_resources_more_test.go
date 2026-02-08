package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestAddGroupResourcesSuccess(t *testing.T) {
	namespace := "default"
	widget := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "w1", "namespace": namespace},
	}}
	cluster := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Cluster",
		"metadata":   map[string]any{"name": "c1"},
	}}
	gvrWidget := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "widgets"}
	gvrCluster := schema.GroupVersionResource{Group: "example.com", Version: "v1", Resource: "clusters"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrWidget:  "WidgetList",
		gvrCluster: "ClusterList",
	}, widget, cluster)

	discoveryClient := &meshDiscovery{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "example.com/v1", APIResources: []metav1.APIResource{
				{Name: "widgets", Kind: "Widget", Namespaced: true},
				{Name: "clusters", Kind: "Cluster", Namespaced: false},
			}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "example.com"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper},
		Policy:  policy.NewAuthorizer(),
	})
	graph := newGraphBuilder()
	if warnings := toolset.addGroupResources(context.Background(), graph, namespace, "example.com", map[string]string{}, nil); len(warnings) != 0 {
		t.Fatalf("unexpected group resource warnings: %v", warnings)
	}
	if len(graph.nodes) == 0 {
		t.Fatalf("expected group resources to add nodes")
	}
}
