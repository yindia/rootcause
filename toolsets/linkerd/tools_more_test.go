package linkerd

import (
	"context"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	appsv1 "k8s.io/api/apps/v1"
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

type linkerdDiscoveryResources struct {
	resources []*metav1.APIResourceList
	groups    *metav1.APIGroupList
}

func (d *linkerdDiscoveryResources) ServerGroups() (*metav1.APIGroupList, error) {
	return d.groups, nil
}

func (d *linkerdDiscoveryResources) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, res := range d.resources {
		if res.GroupVersion == groupVersion {
			return res, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (d *linkerdDiscoveryResources) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	out := make([]*metav1.APIGroup, 0, len(d.groups.Groups))
	for i := range d.groups.Groups {
		group := d.groups.Groups[i]
		out = append(out, &group)
	}
	return out, d.resources, nil
}

func (d *linkerdDiscoveryResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *linkerdDiscoveryResources) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *linkerdDiscoveryResources) ServerVersion() (*version.Info, error) {
	return &version.Info{GitVersion: "v1.27.0", Major: "1", Minor: "27"}, nil
}

func (d *linkerdDiscoveryResources) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *linkerdDiscoveryResources) OpenAPIV3() openapi.Client {
	return nil
}

func (d *linkerdDiscoveryResources) RESTClient() rest.Interface {
	return nil
}

func (d *linkerdDiscoveryResources) Fresh() bool {
	return true
}

func (d *linkerdDiscoveryResources) Invalidate() {}

func (d *linkerdDiscoveryResources) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &linkerdDiscoveryResources{}

func newLinkerdToolset(t *testing.T) *Toolset {
	t.Helper()
	replicas := int32(1)
	identity := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "linkerd-identity",
			Namespace: "linkerd",
		},
		Spec: appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	client := k8sfake.NewSimpleClientset(
		identity,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}},
	)

	server := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "policy.linkerd.io/v1beta1",
		"kind":       "Server",
		"metadata": map[string]any{
			"name":      "srv",
			"namespace": "default",
		},
	}}
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "route",
			"namespace": "default",
		},
	}}
	gvrServer := schema.GroupVersionResource{Group: "policy.linkerd.io", Version: "v1beta1", Resource: "servers"}
	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrServer: "ServerList",
		gvrRoute:  "HTTPRouteList",
	}, server, route)

	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "policy.linkerd.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "servers", Kind: "Server", Namespaced: true},
			},
		},
		{
			GroupVersion: "gateway.networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "httproutes", Kind: "HTTPRoute", Namespaced: true},
			},
		},
	}
	discoveryClient := &linkerdDiscoveryResources{
		resources: resources,
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "linkerd.io"},
			{Name: "policy.linkerd.io"},
			{Name: "gateway.networking.k8s.io"},
		}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{
		Typed:     client,
		Dynamic:   dynamicClient,
		Discovery: discoveryClient,
		Mapper:    mapper,
	}
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

func TestLinkerdIdentityAndPolicy(t *testing.T) {
	toolset := newLinkerdToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleIdentityIssues: %v", err)
	}
	if _, err := toolset.handlePolicyDebug(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handlePolicyDebug: %v", err)
	}
}

func TestLinkerdCRStatus(t *testing.T) {
	toolset := newLinkerdToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "Server", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	if _, err := toolset.handleHTTPRouteStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleHTTPRouteStatus: %v", err)
	}
}

func TestLinkerdHelpers(t *testing.T) {
	if inferGroupForKindResource("HTTPRoute", "") != "gateway.networking.k8s.io" {
		t.Fatalf("expected HTTPRoute group")
	}
	if !isGeneralMeshKind("VirtualService", "") {
		t.Fatalf("expected general mesh kind")
	}
	dst := map[string]any{}
	copyArgIfPresent(dst, map[string]any{"name": "demo"}, "name")
	if dst["name"] != "demo" {
		t.Fatalf("expected copyArgIfPresent to copy value")
	}
}
