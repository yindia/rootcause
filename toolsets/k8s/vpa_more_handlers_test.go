package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

func TestHandleVPADebugNoResources(t *testing.T) {
	namespace := "default"
	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "VerticalPodAutoscalerList"})
	resources := []*metav1.APIResourceList{{
		GroupVersion: "autoscaling.k8s.io/v1",
		APIResources: []metav1.APIResource{{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
	}}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{Name: "autoscaling.k8s.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
			},
		},
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Dynamic: dyn, Discovery: cached, Mapper: mapper},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleVPADebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace},
	}); err != nil {
		t.Fatalf("handleVPADebug no resources: %v", err)
	}
}

func TestHandleVPADebugNameNotFound(t *testing.T) {
	namespace := "default"
	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "VerticalPodAutoscalerList"})
	resources := []*metav1.APIResourceList{{
		GroupVersion: "autoscaling.k8s.io/v1",
		APIResources: []metav1.APIResource{{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
	}}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{Name: "autoscaling.k8s.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
			},
		},
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Dynamic: dyn, Discovery: cached, Mapper: mapper},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleVPADebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": namespace, "name": "missing"},
	}); err != nil {
		t.Fatalf("handleVPADebug not found: %v", err)
	}
}
