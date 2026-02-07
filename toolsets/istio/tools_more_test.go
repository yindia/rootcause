package istio

import (
	"bytes"
	"context"
	"io"
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
	clienttesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type staticResponse struct {
	raw []byte
	err error
}

func (r staticResponse) DoRaw(context.Context) ([]byte, error) {
	return r.raw, r.err
}

func (r staticResponse) Stream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(r.raw)), r.err
}

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
	gwAPI := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "gw-api",
			"namespace": "default",
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
	gvrGateway := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS:      "VirtualServiceList",
		gvrDR:      "DestinationRuleList",
		gvrGW:      "GatewayList",
		gvrSE:      "ServiceEntryList",
		gvrGateway: "GatewayList",
		gvrRoute:   "HTTPRouteList",
	}, vs, dr, gw, se, gwAPI, route)

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
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
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
	if _, err := toolset.handleDestinationRuleStatus(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default", "name": "dr-api"},
	}); err != nil {
		t.Fatalf("handleDestinationRuleStatus: %v", err)
	}
	if _, err := toolset.handleGatewayStatus(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default", "name": "gw-api"},
	}); err != nil {
		t.Fatalf("handleGatewayStatus: %v", err)
	}
	if _, err := toolset.handleHTTPRouteStatus(ctx, mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default", "name": "route-api"},
	}); err != nil {
		t.Fatalf("handleHTTPRouteStatus: %v", err)
	}
	if toolset.Version() == "" {
		t.Fatalf("expected toolset version")
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
	_, err = toolset.handleProxyListeners(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	})
	if err == nil {
		t.Fatalf("expected proxy listeners error for missing args")
	}
	_, _ = toolset.handleProxyRoutes(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	_, _ = toolset.handleProxyEndpoints(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	_, _ = toolset.handleProxyBootstrap(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	_, _ = toolset.handleProxyConfigDump(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
}

func TestIstioProxyAdminSuccess(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "istio-proxy"}}},
	}
	client := k8sfake.NewSimpleClientset(
		pod,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
	client.Fake.PrependProxyReactor("pods", func(action clienttesting.Action) (bool, rest.ResponseWrapper, error) {
		return true, staticResponse{raw: []byte(`{"ok":true}`)}, nil
	})
	clients := &kube.Clients{Typed: client}
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
	result, err := toolset.handleProxyClusters(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "proxy",
		},
	})
	if err != nil {
		t.Fatalf("handleProxyClusters: %v", err)
	}
	if result.Data.(map[string]any)["evidence"] == nil {
		t.Fatalf("expected proxy evidence")
	}
}

func TestIstioListHelpers(t *testing.T) {
	toolset := newIstioToolset(t)
	ctx := context.Background()
	user := policy.User{Role: policy.RoleCluster}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	if _, _, err := toolset.listObjects(ctx, user, gvr, true, "", ""); err != nil {
		t.Fatalf("listObjects cluster: %v", err)
	}
	if _, _, err := toolset.listObjects(ctx, user, gvr, false, "", ""); err != nil {
		t.Fatalf("listObjects cluster scope: %v", err)
	}
	nsUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, namespaces, err := toolset.listObjects(ctx, nsUser, gvr, true, "", ""); err != nil || len(namespaces) == 0 {
		t.Fatalf("listObjects namespace: %v", err)
	}
	if _, namespaces, err := toolset.listObjects(ctx, nsUser, gvr, true, "default", ""); err != nil || len(namespaces) == 0 {
		t.Fatalf("listObjects namespace scoped: %v", err)
	}
	if _, _, err := toolset.listServices(ctx, nsUser, "default"); err != nil {
		t.Fatalf("listServices namespace: %v", err)
	}
	if _, _, err := toolset.listServices(ctx, user, ""); err != nil {
		t.Fatalf("listServices cluster: %v", err)
	}
	if _, _, err := toolset.listServices(ctx, nsUser, ""); err != nil {
		t.Fatalf("listServices namespace list: %v", err)
	}
}

func TestIstioCRStatusNameWithoutNamespace(t *testing.T) {
	toolset := newIstioToolset(t)
	_, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "name": "vs-api"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus name search: %v", err)
	}
}

func TestIstioProxyAdminNotFoundAndNoProxy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	client := k8sfake.NewSimpleClientset(
		pod,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
	clients := &kube.Clients{Typed: client}
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
	if _, err := toolset.handleProxyAdmin(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "missing",
		},
	}, "clusters"); err != nil {
		t.Fatalf("expected not found handling, got error: %v", err)
	}
	if _, err := toolset.handleProxyAdmin(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace": "default",
			"pod":       "app",
		},
	}, "clusters"); err != nil {
		t.Fatalf("expected no-proxy handling, got error: %v", err)
	}
}

func TestIstioInitAndToInt(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected init error without clients")
	}
	if toInt("15000", 15000) != 15000 {
		t.Fatalf("expected string toInt fallback")
	}
	if toInt(float64(9000), 15000) != 9000 {
		t.Fatalf("expected float toInt")
	}
	if toInt(int64(7000), 15000) != 7000 {
		t.Fatalf("expected int64 toInt")
	}
	if toInt(3000, 15000) != 3000 {
		t.Fatalf("expected int toInt")
	}
	if toInt("bad", 15) != 15 {
		t.Fatalf("expected toInt fallback on parse error")
	}
}

func TestHandlePodsByServiceBranches(t *testing.T) {
	serviceNoSelector := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "noselector", Namespace: "default"},
	}
	client := k8sfake.NewSimpleClientset(
		serviceNoSelector,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
	clients := &kube.Clients{Typed: client}
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
	if _, err := toolset.handlePodsByService(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default", "service": "missing"},
	}); err != nil {
		t.Fatalf("expected missing service handled, got: %v", err)
	}
	if _, err := toolset.handlePodsByService(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default", "service": "noselector"},
	}); err != nil {
		t.Fatalf("expected no selector handled, got: %v", err)
	}
}

func TestHandleCRStatusNotFoundAndMultipleNamespaces(t *testing.T) {
	vsA := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "shared",
			"namespace": "a",
		},
	}}
	vsB := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "shared",
			"namespace": "b",
		},
	}}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "VirtualServiceList",
	}, vsA, vsB)
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
			},
		},
	}
	discoveryClient := &istioDiscoveryResources{
		resources: resources,
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	)
	clients := &kube.Clients{Typed: client, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
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
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "name": "missing", "namespace": "a"},
	}); err != nil {
		t.Fatalf("expected not found handled, got: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "name": "shared"},
	}); err == nil {
		t.Fatalf("expected error for multiple namespaces")
	}
}

func TestHandleCRStatusListAllNamespaces(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus list all namespaces: %v", err)
	}
}

func TestHandleCRStatusAllowedNamespaces(t *testing.T) {
	toolset := newIstioToolset(t)
	user := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus allowed namespaces: %v", err)
	}
}

func TestHandleCRStatusMissingKind(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{},
	}); err == nil {
		t.Fatalf("expected error for missing kind/resource")
	}
}

func TestHandleCRStatusWithResource(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"resource": "virtualservices", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus with resource: %v", err)
	}
}

func TestHandleCRStatusSearchNotFound(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "name": "absent"},
	}); err != nil {
		t.Fatalf("expected search not found handled: %v", err)
	}
}

func TestHandleCRStatusWithSelector(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "labelSelector": "app=missing"},
	}); err != nil {
		t.Fatalf("handleCRStatus selector: %v", err)
	}
}

func TestHandleCRStatusWithStatusField(t *testing.T) {
	vs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-status",
			"namespace": "default",
		},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
		},
	}}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "VirtualServiceList",
	}, vs)
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}},
		},
	}
	discoveryClient := &istioDiscoveryResources{
		resources: resources,
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}})
	clients := &kube.Clients{Typed: client, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
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
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "name": "vs-status", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus status field: %v", err)
	}
}

func TestIstioHelperFunctions(t *testing.T) {
	if !hostMatchesPattern("api.example.com", "*.example.com") {
		t.Fatalf("expected wildcard match")
	}
	if hostMatchesPattern("api.example.com", "other.com") {
		t.Fatalf("expected host mismatch")
	}
	if !hostMatchesPattern("api.example.com", "api.*") {
		t.Fatalf("expected prefix match")
	}
	if !hostMatchesPattern("api.example.com", "*") {
		t.Fatalf("expected catch-all match")
	}
	if !hostMatchesPattern("api.example.com", "*.com") {
		t.Fatalf("expected suffix match")
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

func TestHandleConfigSummaryNotDetected(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &istioDiscoveryResources{groups: &metav1.APIGroupList{}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})
	if _, err := toolset.handleConfigSummary(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleConfigSummary not detected: %v", err)
	}
}

func TestHandleExternalDependencyNoHosts(t *testing.T) {
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}},
	)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{},
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Dynamic: dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})
	if _, err := toolset.handleExternalDependencyCheck(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleExternalDependencyCheck no hosts: %v", err)
	}
}

func TestHandleConfigSummaryNamespace(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleConfigSummary(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleConfigSummary namespace: %v", err)
	}
}
