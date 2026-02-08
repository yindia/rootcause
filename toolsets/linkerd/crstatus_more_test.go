package linkerd

import (
	"context"
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
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type policyDiscovery struct{}

func (d *policyDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "policy.linkerd.io"}}}, nil
}
func (d *policyDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return &metav1.APIResourceList{}, nil
}
func (d *policyDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (d *policyDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *policyDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *policyDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *policyDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *policyDiscovery) OpenAPIV3() openapi.Client { return nil }
func (d *policyDiscovery) RESTClient() rest.Interface { return nil }
func (d *policyDiscovery) Fresh() bool { return true }
func (d *policyDiscovery) Invalidate() {}
func (d *policyDiscovery) WithLegacy() discovery.DiscoveryInterface { return d }

var _ discovery.CachedDiscoveryInterface = &policyDiscovery{}

func newCRToolset(t *testing.T, objects ...runtime.Object) *Toolset {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrServer := schema.GroupVersionResource{Group: "policy.linkerd.io", Version: "v1beta1", Resource: "servers"}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrServer: "ServerList",
	}, objects...)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "policy.linkerd.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "policy.linkerd.io/v1beta1", Version: "v1beta1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "policy.linkerd.io/v1beta1", Version: "v1beta1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {{Name: "servers", Kind: "Server", Namespaced: true}},
			},
		},
	})
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "policy.linkerd.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "servers", Kind: "Server", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "policy.linkerd.io"}}},
	}
	typed := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	)
	clients := &kube.Clients{Typed: typed, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
	cfg := config.DefaultConfig()
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Evidence: evidence.NewCollector(clients),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return toolset
}

func newVirtualServiceToolset(t *testing.T, groups []metav1.APIGroup, objects ...runtime.Object) *Toolset {
	t.Helper()
	scheme := runtime.NewScheme()
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS: "VirtualServiceList",
	}, objects...)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "networking.istio.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "networking.istio.io/v1beta1", Version: "v1beta1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "networking.istio.io/v1beta1", Version: "v1beta1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}},
			},
		},
	})
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "networking.istio.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: groups},
	}
	typed := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: typed, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
	cfg := config.DefaultConfig()
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Evidence: evidence.NewCollector(clients),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return toolset
}

func TestLinkerdCRStatusMissingKind(t *testing.T) {
	toolset := newCRToolset(t)
	_, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err == nil {
		t.Fatalf("expected error for missing kind")
	}
}

func TestLinkerdCRStatusNotFound(t *testing.T) {
	toolset := newCRToolset(t)
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "name": "missing", "namespace": "default"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected status evidence for not found")
	}
}

func TestLinkerdCRStatusMultipleNamespaces(t *testing.T) {
	objA := &unstructured.Unstructured{}
	objA.SetAPIVersion("policy.linkerd.io/v1beta1")
	objA.SetKind("Server")
	objA.SetName("dup")
	objA.SetNamespace("a")
	objB := objA.DeepCopy()
	objB.SetNamespace("b")
	toolset := newCRToolset(t, objA, objB)
	_, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "name": "dup"},
	})
	if err == nil {
		t.Fatalf("expected error for multiple namespaces")
	}
}

func TestLinkerdCRStatusSingleNamespaceMatch(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("policy.linkerd.io/v1beta1")
	obj.SetKind("Server")
	obj.SetName("single")
	obj.SetNamespace("default")
	toolset := newCRToolset(t, obj)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "name": "single"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
}

func TestLinkerdCRStatusListNoMatches(t *testing.T) {
	toolset := newCRToolset(t)
	toolset.ctx.Clients.Discovery = &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "policy.linkerd.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "servers", Kind: "Server", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{},
	}
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "labelSelector": "app=missing"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected no matching resources evidence")
	}
}

func TestLinkerdCRStatusAPIVersionError(t *testing.T) {
	toolset := newCRToolset(t)
	_, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"apiVersion": "bad/v1", "kind": "Server"},
	})
	if err == nil {
		t.Fatalf("expected error for bad apiVersion")
	}
}

func TestLinkerdCRStatusGeneralFetchNotDetected(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("networking.istio.io/v1beta1")
	obj.SetKind("VirtualService")
	obj.SetName("api")
	obj.SetNamespace("default")
	toolset := newVirtualServiceToolset(t, nil, obj)
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "linkerdDetected" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected linkerdDetected evidence")
	}
}

func TestLinkerdCRStatusNamespaceList(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("policy.linkerd.io/v1beta1")
	obj.SetKind("Server")
	obj.SetName("ns-list")
	obj.SetNamespace("default")
	toolset := newCRToolset(t, obj)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
}

func TestLinkerdCRStatusNameWithoutNamespaceNotFound(t *testing.T) {
	toolset := newCRToolset(t)
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "name": "missing"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected status evidence for missing")
	}
}

func TestLinkerdCRStatusStatusMap(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("policy.linkerd.io/v1beta1")
	obj.SetKind("Server")
	obj.SetName("with-status")
	obj.SetNamespace("default")
	obj.Object["status"] = map[string]any{"phase": "Ready"}
	toolset := newCRToolset(t, obj)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Server", "name": "with-status", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
}

func TestLinkerdPolicyDebugGroupPresent(t *testing.T) {
	toolset := newMinimalToolset(t, &policyDiscovery{}, k8sfake.NewSimpleClientset())
	if _, err := toolset.handlePolicyDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handlePolicyDebug: %v", err)
	}
}
