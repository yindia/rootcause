package k8s

import (
	"context"
	"errors"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/apimachinery/pkg/version"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type errorDiscovery struct{}

func (d *errorDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return &metav1.APIGroupList{}, nil }
func (d *errorDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return &metav1.APIResourceList{}, nil
}
func (d *errorDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (d *errorDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, errors.New("boom")
}
func (d *errorDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *errorDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *errorDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *errorDiscovery) OpenAPIV3() openapi.Client                 { return nil }
func (d *errorDiscovery) RESTClient() rest.Interface                 { return nil }
func (d *errorDiscovery) Fresh() bool                                { return true }
func (d *errorDiscovery) Invalidate()                                {}
func (d *errorDiscovery) WithLegacy() discovery.DiscoveryInterface   { return d }

func TestGraphNestedNilAndPrincipalParsing(t *testing.T) {
	if got := nestedString(nil, "spec"); got != "" {
		t.Fatalf("expected empty nestedString for nil")
	}
	if got := nestedStringSlice(nil, "spec"); got != nil {
		t.Fatalf("expected nil nestedStringSlice for nil")
	}
	if got := nestedStringMap(nil, "spec"); got != nil {
		t.Fatalf("expected nil nestedStringMap for nil")
	}

	if _, _, ok := parseIstioServiceAccountPrincipal("invalid"); ok {
		t.Fatalf("expected invalid principal")
	}
	ns, sa, ok := parseIstioServiceAccountPrincipal("cluster.local/ns/default/sa/app")
	if !ok || ns != "default" || sa != "app" {
		t.Fatalf("unexpected principal parse")
	}
	ns, sa, ok = parseIstioServiceAccountPrincipal("ns/default/sa/app")
	if !ok || ns != "default" || sa != "app" {
		t.Fatalf("unexpected principal parse for short form")
	}
}

func TestIstioAuthorizationPolicyPrincipalsInvalid(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"rules": []any{
				"bad",
				map[string]any{"from": []any{"bad"}},
				map[string]any{"from": []any{map[string]any{"source": "bad"}}},
			},
		},
	}}
	if principals := istioAuthorizationPolicyPrincipals(obj); len(principals) != 0 {
		t.Fatalf("expected no principals from invalid entries")
	}
}

func TestAddNetworkPolicyPeerEdgesInvalidSelector(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()
	peer := networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"bad key": "value"}},
	}
	warnings := []string{}
	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", "policy", peer, "allows-from", nil, &warnings)
	if len(warnings) == 0 {
		t.Fatalf("expected warning for invalid namespace selector")
	}
}

func TestAddGroupResourcesWarnings(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: &errorDiscovery{}},
		Policy:  policy.NewAuthorizer(),
	})

	graph := newGraphBuilder()
	warnings := toolset.addGroupResources(context.Background(), graph, "default", "example.com", map[string]string{}, nil)
	if len(warnings) == 0 {
		t.Fatalf("expected warnings for missing cluster resource list kind")
	}
}

func TestAddGatewayAndIstioGraphWarnings(t *testing.T) {
	namespace := "default"
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
		"spec":       map[string]any{"hosts": []any{"api.default.svc"}},
	}}
	istioGateway := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "Gateway",
		"metadata":   map[string]any{"name": "istio-gw", "namespace": namespace},
	}}
	destinationRule := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "DestinationRule",
		"metadata":   map[string]any{"name": "dr", "namespace": namespace},
		"spec":       map[string]any{"host": "api.default.svc"},
	}}

	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	gvrGateway := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrIstioGateway := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "gateways"}
	gvrDR := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrGateway: "GatewayList",
		gvrRoute: "HTTPRouteList",
		gvrVS:    "VirtualServiceList",
		gvrIstioGateway: "GatewayList",
		gvrDR:           "DestinationRuleList",
	}, gateway, httpRoute, virtualService, istioGateway, destinationRule)

	discoveryClient := &meshDiscovery{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
				{Name: "httproutes", Kind: "HTTPRoute", Namespaced: true},
			}},
			{GroupVersion: "networking.istio.io/v1beta1", APIResources: []metav1.APIResource{
				{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
				{Name: "destinationrules", Kind: "DestinationRule", Namespaced: true},
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
			}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "gateway.networking.k8s.io"},
			{Name: "networking.istio.io"},
		}},
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
	_ = toolset.addGatewayAPIGraph(context.Background(), graph, namespace, map[string]string{"api": "api"})
	_ = toolset.addIstioGraph(context.Background(), graph, namespace, map[string]string{"api": "api"})
}

func TestAddServiceGraphPodNotFound(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "api"}},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Subsets: []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "missing"}}}},
		},
	}
	cache := newGraphCache()
	cache.servicesLoaded = true
	cache.services["api"] = service
	cache.serviceList = append(cache.serviceList, service)
	cache.endpointsLoaded = true
	cache.endpoints["api"] = endpoints
	cache.endpointList = append(cache.endpointList, endpoints)
	cache.podsLoaded = true

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()
	if _, err := toolset.addServiceGraph(context.Background(), graph, namespace, "api", cache); err != nil {
		t.Fatalf("addServiceGraph pod not found: %v", err)
	}
}

func TestAddDeploymentAndDaemonSetSelectorErrors(t *testing.T) {
	namespace := "default"
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "Invalid"}}},
		},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: namespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "Invalid"}}},
		},
	}

	cache := newGraphCache()
	cache.deploymentsLoaded = true
	cache.deployments["api"] = deploy
	cache.deploymentList = append(cache.deploymentList, deploy)
	cache.daemonsetsLoaded = true
	cache.daemonsets["agent"] = ds
	cache.daemonsetList = append(cache.daemonsetList, ds)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()

	if _, err := toolset.addDeploymentGraph(context.Background(), graph, namespace, "api", cache); err == nil {
		t.Fatalf("expected deployment selector error")
	}
	if _, err := toolset.addDaemonSetGraph(context.Background(), graph, namespace, "agent", cache); err == nil {
		t.Fatalf("expected daemonset selector error")
	}
}

func TestGetPodNotFoundCache(t *testing.T) {
	cache := newGraphCache()
	cache.podsLoaded = true
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.getPod(context.Background(), cache, "default", "missing"); err == nil {
		t.Fatalf("expected getPod not found")
	}
}

func TestAddReplicaSetPodsSelectorError(t *testing.T) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "rs", Namespace: "default"},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: "Invalid"}}},
		},
	}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.addReplicaSetPods(context.Background(), newGraphBuilder(), "default", rs, newGraphCache()); err == nil {
		t.Fatalf("expected selector error")
	}
}

func TestIngressBackendServices(t *testing.T) {
	ingress := &networkingv1.Ingress{
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{Name: "default"},
			},
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api"}}},
							},
						},
					},
				},
			},
		},
	}
	services := ingressBackendServices(ingress)
	if len(services) != 2 {
		t.Fatalf("expected backend services to include default and rule")
	}
}
