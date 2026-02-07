package k8s

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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

type stubCollector struct{}

func (stubCollector) EventsForObject(context.Context, *unstructured.Unstructured) ([]corev1.Event, error) {
	return nil, nil
}

func (stubCollector) OwnerChain(context.Context, *unstructured.Unstructured) ([]string, error) {
	return nil, nil
}

func (stubCollector) PodStatusSummary(*corev1.Pod) map[string]any {
	return map[string]any{}
}

func (stubCollector) RelatedPods(context.Context, string, labels.Selector) ([]corev1.Pod, error) {
	return nil, nil
}

func (stubCollector) EndpointsForService(context.Context, string, string) (*corev1.Endpoints, error) {
	return nil, nil
}

func (stubCollector) ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", gvr.Resource, name)
	}
	return fmt.Sprintf("%s/%s/%s", gvr.Resource, namespace, name)
}

func newTestToolset(objects ...*unstructured.Unstructured) (*Toolset, schema.GroupVersionResource) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	scheme := runtime.NewScheme()
	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "PodList",
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
				"v1": {{Name: "pods", Kind: "Pod", Namespaced: true}},
			},
		},
	})

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Dynamic: dyn, Mapper: mapper},
		Policy:   policy.NewAuthorizer(),
		Evidence: stubCollector{},
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	return toolset, gvr
}

func TestHandleGetPod(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("demo")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "demo",
			"namespace":  "default",
		},
		User: policy.User{Role: policy.RoleCluster},
	}
	result, err := toolset.handleGet(context.Background(), req)
	if err != nil {
		t.Fatalf("handleGet: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	meta, ok := data["metadata"].(map[string]any)
	if !ok || meta["name"] != "demo" {
		t.Fatalf("unexpected metadata: %#v", data["metadata"])
	}
}

func TestHandleListPods(t *testing.T) {
	pod1 := &unstructured.Unstructured{}
	pod1.SetAPIVersion("v1")
	pod1.SetKind("Pod")
	pod1.SetName("pod-1")
	pod1.SetNamespace("default")

	pod2 := &unstructured.Unstructured{}
	pod2.SetAPIVersion("v1")
	pod2.SetKind("Pod")
	pod2.SetName("pod-2")
	pod2.SetNamespace("default")

	toolset, _ := newTestToolset(pod1, pod2)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"resources": []any{
				map[string]any{"apiVersion": "v1", "kind": "Pod"},
			},
			"namespace": "default",
		},
		User: policy.User{Role: policy.RoleCluster},
	}
	result, err := toolset.handleList(context.Background(), req)
	if err != nil {
		t.Fatalf("handleList: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	resultsAny := data["results"]
	var results []map[string]any
	switch v := resultsAny.(type) {
	case []map[string]any:
		results = v
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				results = append(results, m)
			}
		}
	default:
		t.Fatalf("unexpected results: %#v", data["results"])
	}
	if len(results) != 1 {
		t.Fatalf("unexpected results length: %#v", results)
	}
	itemsAny := results[0]["items"]
	var items []any
	switch v := itemsAny.(type) {
	case []any:
		items = v
	case []map[string]any:
		for _, item := range v {
			items = append(items, item)
		}
	default:
		t.Fatalf("unexpected items: %#v", results[0]["items"])
	}
	if len(items) != 2 {
		t.Fatalf("unexpected items length: %#v", items)
	}
}

func TestHandleDescribePod(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("demo")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "demo",
			"namespace":  "default",
		},
		User: policy.User{Role: policy.RoleCluster},
	}
	result, err := toolset.handleDescribe(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribe: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence in describe output")
	}
}

func TestHandleDeletePod(t *testing.T) {
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetName("demo")
	pod.SetNamespace("default")

	toolset, _ := newTestToolset(pod)
	req := mcp.ToolRequest{
		Arguments: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "demo",
			"namespace":  "default",
			"confirm":    true,
		},
		User: policy.User{Role: policy.RoleCluster},
	}
	result, err := toolset.handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["deleted"] != true {
		t.Fatalf("unexpected delete result: %#v", result.Data)
	}
}
