package istio

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

type istioDiscoveryResources struct {
	resources []*metav1.APIResourceList
	groups    *metav1.APIGroupList
}

func (d *istioDiscoveryResources) ServerGroups() (*metav1.APIGroupList, error) {
	return d.groups, nil
}

func (d *istioDiscoveryResources) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, res := range d.resources {
		if res.GroupVersion == groupVersion {
			return res, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (d *istioDiscoveryResources) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	out := make([]*metav1.APIGroup, 0, len(d.groups.Groups))
	for i := range d.groups.Groups {
		group := d.groups.Groups[i]
		out = append(out, &group)
	}
	return out, d.resources, nil
}

func (d *istioDiscoveryResources) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *istioDiscoveryResources) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}

func (d *istioDiscoveryResources) ServerVersion() (*version.Info, error) {
	return &version.Info{GitVersion: "v1.27.0", Major: "1", Minor: "27"}, nil
}

func (d *istioDiscoveryResources) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *istioDiscoveryResources) OpenAPIV3() openapi.Client {
	return nil
}

func (d *istioDiscoveryResources) RESTClient() rest.Interface {
	return nil
}

func (d *istioDiscoveryResources) Fresh() bool {
	return true
}

func (d *istioDiscoveryResources) Invalidate() {}

func (d *istioDiscoveryResources) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &istioDiscoveryResources{}

func newIstioToolset(t *testing.T) *Toolset {
	t.Helper()
	istiodReplicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istiod",
			Namespace: "istio-system",
			Labels:    map[string]string{"app": "istiod"},
		},
		Spec: appsv1.DeploymentSpec{Replicas: &istiodReplicas},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}
	proxyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-pod",
			Namespace: "default",
			Labels:    map[string]string{"app": "api"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}, {Name: "istio-proxy"}},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionFalse},
			},
		},
	}
	apiPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "api"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}, {Name: "istio-proxy"}},
		},
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "api"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80},
			},
		},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.1"}}},
		},
	}
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}},
		deploy,
		proxyPod,
		apiPod,
		service,
		endpoints,
	)

	vs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-api",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hosts": []any{"api.default.svc.cluster.local", "missing.example.com"},
		},
	}}
	dr := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "DestinationRule",
		"metadata": map[string]any{
			"name":      "dr-api",
			"namespace": "default",
		},
		"spec": map[string]any{
			"host": "api.default.svc.cluster.local",
		},
	}}
	gw := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "gw-api",
			"namespace": "default",
		},
		"spec": map[string]any{
			"servers": []any{
				map[string]any{"hosts": []any{"gateway.example.com"}},
			},
		},
	}}
	se := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "ServiceEntry",
		"metadata": map[string]any{
			"name":      "se-ext",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hosts": []any{"external.example.com"},
		},
	}}
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]any{
			"name":      "route-api",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hostnames": []any{"route.example.com"},
		},
	}}
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrDR := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	gvrGW := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "gateways"}
	gvrSE := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "serviceentries"}
	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS:    "VirtualServiceList",
		gvrDR:    "DestinationRuleList",
		gvrGW:    "GatewayList",
		gvrSE:    "ServiceEntryList",
		gvrRoute: "HTTPRouteList",
	}, vs, dr, gw, se, route)

	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
				{Name: "destinationrules", Kind: "DestinationRule", Namespaced: true},
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
				{Name: "serviceentries", Kind: "ServiceEntry", Namespaced: true},
			},
		},
		{
			GroupVersion: "gateway.networking.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "httproutes", Kind: "HTTPRoute", Namespaced: true},
			},
		},
	}
	discoveryClient := &istioDiscoveryResources{
		resources: resources,
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "networking.istio.io"},
			{Name: "security.istio.io"},
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

func TestIstioHandlersWithResources(t *testing.T) {
	toolset := newIstioToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	ctx := context.Background()

	if _, err := toolset.handleConfigSummary(ctx, mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleConfigSummary: %v", err)
	}
	if _, err := toolset.handleServiceMeshHosts(ctx, mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleServiceMeshHosts: %v", err)
	}
	if _, err := toolset.handleExternalDependencyCheck(ctx, mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleExternalDependencyCheck: %v", err)
	}
	if _, err := toolset.handleDiscoverNamespaces(ctx, mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleDiscoverNamespaces: %v", err)
	}
	if _, err := toolset.handlePodsByService(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default", "service": "api"},
	}); err != nil {
		t.Fatalf("handlePodsByService: %v", err)
	}
	if _, err := toolset.handleProxyStatus(ctx, mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "VirtualService", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	if _, err := toolset.handleVirtualServiceStatus(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default", "name": "vs-api"},
	}); err != nil {
		t.Fatalf("handleVirtualServiceStatus: %v", err)
	}
}

func TestIstioProxyAdminMissingArgs(t *testing.T) {
	toolset := newIstioToolset(t)
	_, err := toolset.handleProxyClusters(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatalf("expected proxy admin error for missing args")
	}
}

func TestIstioHelperFunctions(t *testing.T) {
	if !hostMatchesPattern("api.example.com", "*.example.com") {
		t.Fatalf("expected wildcard match")
	}
	if hostMatchesPattern("api.example.com", "other.com") {
		t.Fatalf("expected host mismatch")
	}
	payload := parseProxyPayload([]byte(`{"ok":true}`))
	if payloadMap, ok := payload.(map[string]any); !ok || payloadMap["ok"] != true {
		t.Fatalf("unexpected proxy payload: %#v", payload)
	}
	if parseProxyPayload([]byte("raw")) != "raw" {
		t.Fatalf("expected raw payload")
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"servers": []any{map[string]any{"hosts": []any{"example.com"}}},
		},
	}}
	hosts := gatewayServerHosts(obj)
	if len(hosts) != 1 || hosts[0] != "example.com" {
		t.Fatalf("unexpected gateway hosts: %#v", hosts)
	}
	if inferGroupForKindResource("VirtualService", "") != "networking.istio.io" {
		t.Fatalf("expected VirtualService group")
	}
	if !isGatewayAPIKind("HTTPRoute", "") {
		t.Fatalf("expected gateway API kind")
	}
}
