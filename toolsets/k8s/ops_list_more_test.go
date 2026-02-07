package k8s

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newClusterScopeToolset(objects ...*unstructured.Unstructured) *Toolset {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	scheme := runtime.NewScheme()
	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "NamespaceList",
	}, runtimeObjects...)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "namespaces", Kind: "Namespace", Namespaced: false}},
			},
		},
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Dynamic: dyn, Mapper: mapper},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	return toolset
}

func TestHandleListNamespaceRoleAllNamespaces(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("pod-1")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"resources": []any{
				map[string]any{"apiVersion": "v1", "kind": "Pod"},
			},
		},
		User: policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}},
	}
	if _, err := toolset.handleList(context.Background(), req); err != nil {
		t.Fatalf("handleList namespace role: %v", err)
	}
}

func TestHandleListClusterScope(t *testing.T) {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName("demo")
	toolset := newClusterScopeToolset(ns)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"resources": []any{
				map[string]any{"apiVersion": "v1", "kind": "Namespace"},
			},
		},
		User: policy.User{Role: policy.RoleCluster},
	}
	if _, err := toolset.handleList(context.Background(), req); err != nil {
		t.Fatalf("handleList cluster scope: %v", err)
	}
}

func TestHandleListMissingResources(t *testing.T) {
	toolset, _ := newTestToolset()
	if _, err := toolset.handleList(context.Background(), mcp.ToolRequest{}); err == nil {
		t.Fatalf("expected missing resources error")
	}
}
