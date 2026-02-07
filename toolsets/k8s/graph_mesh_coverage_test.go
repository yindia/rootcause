package k8s

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

type meshDiscovery struct {
	resources []*metav1.APIResourceList
	groups    *metav1.APIGroupList
}

func (m *meshDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return m.groups, nil
}
func (m *meshDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, res := range m.resources {
		if res.GroupVersion == groupVersion {
			return res, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}
func (m *meshDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	var groups []*metav1.APIGroup
	if m.groups != nil {
		for i := range m.groups.Groups {
			group := m.groups.Groups[i]
			groups = append(groups, &group)
		}
	}
	return groups, m.resources, nil
}
func (m *meshDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return m.resources, nil
}
func (m *meshDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return m.resources, nil
}
func (m *meshDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (m *meshDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}
func (m *meshDiscovery) OpenAPIV3() openapi.Client { return nil }
func (m *meshDiscovery) RESTClient() rest.Interface { return nil }
func (m *meshDiscovery) Fresh() bool               { return true }
func (m *meshDiscovery) Invalidate()               {}
func (m *meshDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return m
}

var _ discovery.CachedDiscoveryInterface = &meshDiscovery{}

func TestAddMeshGraphCoverage(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: map[string]string{"app": "api"}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	client := k8sfake.NewSimpleClientset(service, pod, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})

	gateway := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata":   map[string]any{"name": "gw", "namespace": namespace},
	}}
	httpRoute := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata":   map[string]any{"name": "route", "namespace": namespace},
		"spec": map[string]any{
			"parentRefs": []any{map[string]any{"name": "gw"}},
			"rules": []any{map[string]any{"backendRefs": []any{map[string]any{"name": "api"}}}},
		},
	}}
	virtualService := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata":   map[string]any{"name": "vs", "namespace": namespace},
		"spec": map[string]any{
			"hosts":    []any{"api.default.svc.cluster.local"},
			"gateways": []any{"gw"},
			"targetRef": map[string]any{
				"kind": "Service",
				"name": "api",
			},
		},
	}}
	destinationRule := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "DestinationRule",
		"metadata":   map[string]any{"name": "dr", "namespace": namespace},
		"spec":       map[string]any{"host": "api.default.svc.cluster.local"},
	}}
	authorizationPolicy := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "security.istio.io/v1beta1",
		"kind":       "AuthorizationPolicy",
		"metadata":   map[string]any{"name": "authz", "namespace": namespace},
		"spec": map[string]any{
			"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
			"rules": []any{map[string]any{"from": []any{map[string]any{"source": map[string]any{"principals": []any{"spiffe://cluster.local/ns/default/sa/api"}}}}}},
		},
	}}
	serviceProfile := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "linkerd.io/v1alpha2",
		"kind":       "ServiceProfile",
		"metadata":   map[string]any{"name": "api.default.svc.cluster.local", "namespace": namespace},
	}}
	serverAuthz := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "policy.linkerd.io/v1alpha1",
		"kind":       "ServerAuthorization",
		"metadata":   map[string]any{"name": "authz", "namespace": namespace},
		"spec":       map[string]any{"server": "api-server"},
	}}

	gvrGateway := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrDR := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	gvrAuthz := schema.GroupVersionResource{Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"}
	gvrProfile := schema.GroupVersionResource{Group: "linkerd.io", Version: "v1alpha2", Resource: "serviceprofiles"}
	gvrServerAuth := schema.GroupVersionResource{Group: "policy.linkerd.io", Version: "v1alpha1", Resource: "serverauthorizations"}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrGateway:   "GatewayList",
		gvrRoute:     "HTTPRouteList",
		gvrVS:        "VirtualServiceList",
		gvrDR:        "DestinationRuleList",
		gvrAuthz:     "AuthorizationPolicyList",
		gvrProfile:   "ServiceProfileList",
		gvrServerAuth: "ServerAuthorizationList",
	}, gateway, httpRoute, virtualService, destinationRule, authorizationPolicy, serviceProfile, serverAuthz)

	discoveryClient := &meshDiscovery{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
				{Name: "httproutes", Kind: "HTTPRoute", Namespaced: true},
			}},
			{GroupVersion: "networking.istio.io/v1beta1", APIResources: []metav1.APIResource{
				{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
				{Name: "destinationrules", Kind: "DestinationRule", Namespaced: true},
			}},
			{GroupVersion: "security.istio.io/v1beta1", APIResources: []metav1.APIResource{
				{Name: "authorizationpolicies", Kind: "AuthorizationPolicy", Namespaced: true},
			}},
			{GroupVersion: "linkerd.io/v1alpha2", APIResources: []metav1.APIResource{
				{Name: "serviceprofiles", Kind: "ServiceProfile", Namespaced: true},
			}},
			{GroupVersion: "policy.linkerd.io/v1alpha1", APIResources: []metav1.APIResource{
				{Name: "serverauthorizations", Kind: "ServerAuthorization", Namespaced: true},
			}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "gateway.networking.k8s.io"},
			{Name: "networking.istio.io"},
			{Name: "security.istio.io"},
			{Name: "linkerd.io"},
			{Name: "policy.linkerd.io"},
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

	graph := newGraphBuilder()
	cache, _ := toolset.buildGraphCache(context.Background(), namespace, true)
	warnings := toolset.addMeshGraph(context.Background(), graph, namespace, cache)
	if len(graph.nodes) == 0 || len(graph.edges) == 0 {
		t.Fatalf("expected mesh graph nodes/edges")
	}
	if warnings == nil {
		t.Fatalf("expected warnings slice")
	}
}
