package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type apiDiscovery struct {
	resourcesByGV map[string]*metav1.APIResourceList
	version       *version.Info
}

func (d *apiDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{}, nil
}

func (d *apiDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	if list, ok := d.resourcesByGV[groupVersion]; ok {
		return list, nil
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (d *apiDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (d *apiDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	lists := make([]*metav1.APIResourceList, 0, len(d.resourcesByGV))
	for _, list := range d.resourcesByGV {
		lists = append(lists, list)
	}
	return lists, nil
}

func (d *apiDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return d.ServerPreferredResources()
}

func (d *apiDiscovery) ServerVersion() (*version.Info, error) {
	if d.version != nil {
		return d.version, nil
	}
	return &version.Info{}, nil
}

func (d *apiDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *apiDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *apiDiscovery) RESTClient() rest.Interface {
	return nil
}

func (d *apiDiscovery) Fresh() bool {
	return true
}

func (d *apiDiscovery) Invalidate() {}

func (d *apiDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &apiDiscovery{}

func TestHandleAPIResources(t *testing.T) {
	discoveryClient := &apiDiscovery{
		resourcesByGV: map[string]*metav1.APIResourceList{
			"v1": {
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod"}},
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
	result, err := toolset.handleAPIResources(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"query": "pod"},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleAPIResources: %v", err)
	}
	data := result.Data.(map[string]any)
	if matched, ok := data["matched"].(int); ok && matched == 0 {
		t.Fatalf("expected matched resources")
	}
}

func TestHandleCRDs(t *testing.T) {
	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name": "widgets.example.com",
		},
		"spec": map[string]any{
			"group": "example.com",
			"scope": "Namespaced",
			"names": map[string]any{
				"kind":   "Widget",
				"plural": "widgets",
				"shortNames": []any{
					"wdg",
				},
			},
			"versions": []any{
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		crdGVR: "CustomResourceDefinitionList",
	}, crd)

	toolset := New()
	cfg := config.DefaultConfig()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dyn},
		Policy:  policy.NewAuthorizer(),
	})
	result, err := toolset.handleCRDs(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"query": "widget"},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleCRDs: %v", err)
	}
	data := result.Data.(map[string]any)
	if matched, ok := data["matched"].(int); ok && matched == 0 {
		t.Fatalf("expected matched crds")
	}
}

func TestHandleContext(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = kubeconfigPath
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{},
	})
	result, err := toolset.handleContext(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"action": "current"},
	})
	if err != nil {
		t.Fatalf("handleContext: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["current"] != "test" {
		t.Fatalf("unexpected current context: %#v", data)
	}
}

func TestHandleContextUseAction(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{},
	})
	_, err := toolset.handleContext(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"action": "use"},
	})
	if err == nil {
		t.Fatalf("expected use action error")
	}
}

func TestHandleEvents(t *testing.T) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "default"},
		Reason:     "Started",
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	client := k8sfake.NewSimpleClientset(event, ns)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Typed: client},
		Policy:  policy.NewAuthorizer(),
	})
	result, err := toolset.handleEvents(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"namespace": "default"},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleEvents: %v", err)
	}
	data := result.Data.(map[string]any)
	if events, ok := data["events"].([]corev1.Event); ok && len(events) == 0 {
		t.Fatalf("expected events")
	}
}

func TestHandleExplain(t *testing.T) {
	discoveryClient := &apiDiscovery{
		resourcesByGV: map[string]*metav1.APIResourceList{
			"v1": {
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod", Namespaced: true}},
			},
		},
	}
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
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: discoveryClient, Mapper: mapper},
	})
	result, err := toolset.handleExplain(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{"apiVersion": "v1", "kind": "Pod"},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["resource"] != "pods" {
		t.Fatalf("unexpected explain data: %#v", data)
	}
}

func TestHandlePing(t *testing.T) {
	discoveryClient := &apiDiscovery{version: &version.Info{GitVersion: "v1.28.0"}}
	toolset := New()
	cfg := config.DefaultConfig()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: discoveryClient},
	})
	result, err := toolset.handlePing(context.Background(), mcp.ToolRequest{})
	if err != nil {
		t.Fatalf("handlePing: %v", err)
	}
	data := result.Data.(map[string]any)
	if ok, _ := data["ok"].(bool); !ok {
		t.Fatalf("expected ok=true")
	}
}
