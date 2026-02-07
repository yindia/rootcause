package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newDynamicToolset(gvr schema.GroupVersionResource, listKind, kind string, objects ...*unstructured.Unstructured) *Toolset {
	scheme := runtime.NewScheme()
	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: listKind}, runtimeObjects...)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: gvr.Group,
				Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: gvr.GroupVersion().String(), Version: gvr.Version}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: gvr.GroupVersion().String(), Version: gvr.Version},
			},
			VersionedResources: map[string][]metav1.APIResource{
				gvr.Version: {{Name: gvr.Resource, Kind: kind, Namespaced: true}},
			},
		},
	})
	client := k8sfake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client, Dynamic: dyn, Mapper: mapper}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	})
	return toolset
}

func TestHandleCreateConfigMap(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	toolset := newDynamicToolset(gvr, "ConfigMapList", "ConfigMap")
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  namespace: default\n"
	result, err := toolset.handleCreate(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"manifest": manifest,
			"confirm":  true,
		},
	})
	if err != nil {
		t.Fatalf("handleCreate: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["created"] == nil {
		t.Fatalf("expected created output")
	}
}

func TestHandleRolloutStatus(t *testing.T) {
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1},
	}
	client := k8sfake.NewSimpleClientset(deploy)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	_, err := toolset.handleRollout(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"action":    "status",
			"name":      "api",
			"namespace": "default",
			"confirm":   true,
		},
	})
	if err != nil {
		t.Fatalf("handleRollout: %v", err)
	}
}

func TestHandleContextCurrent(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "kubeconfig")
	kubeconfig := "apiVersion: v1\nkind: Config\nclusters:\n- name: test\n  cluster:\n    server: https://example.com\ncontexts:\n- name: test\n  context:\n    cluster: test\n    user: test\ncurrent-context: test\nusers:\n- name: test\n  user:\n    token: fake\n"
	if err := os.WriteFile(configPath, []byte(kubeconfig), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Kubeconfig = configPath
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	result, err := toolset.handleContext(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"action": "current"},
	})
	if err != nil {
		t.Fatalf("handleContext: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["current"] != "test" {
		t.Fatalf("unexpected current context: %v", data["current"])
	}
}

func TestHandleExplainResourceInfo(t *testing.T) {
	resources := []*metav1.APIResourceList{{
		GroupVersion: "apps/v1",
		APIResources: []metav1.APIResource{{Name: "deployments", Kind: "Deployment", Namespaced: true}},
	}}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "apps/v1", Version: "v1"}},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "deployments", Kind: "Deployment", Namespaced: true}},
			},
		},
	})
	clients := &kube.Clients{Discovery: cached, Mapper: mapper}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	result, err := toolset.handleExplain(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Deployment", "apiVersion": "apps/v1"},
	})
	if err != nil {
		t.Fatalf("handleExplain: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["resource"] != "deployments" {
		t.Fatalf("unexpected resource: %v", data["resource"])
	}
}

func TestHandleGenericUnsupported(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	_, err := toolset.handleGeneric(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"verb": "unknown"}})
	if err == nil {
		t.Fatalf("expected error for unsupported verb")
	}
}
