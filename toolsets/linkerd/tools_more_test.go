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
	controlPlane := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "linkerd-controller",
			Namespace: "linkerd",
			Labels: map[string]string{
				linkerdSelector: "true",
			},
		},
		Spec: appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}
	proxyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-proxy",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "linkerd-proxy"}}},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		},
	}
	client := k8sfake.NewSimpleClientset(
		identity,
		controlPlane,
		proxyPod,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
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
	virtualService := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "api",
			"namespace": "default",
		},
	}}
	virtualServiceMulti := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "multi",
			"namespace": "default",
		},
	}}
	virtualServiceOther := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "multi",
			"namespace": "other",
		},
	}}
	destinationRule := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "DestinationRule",
		"metadata": map[string]any{
			"name":      "api",
			"namespace": "default",
		},
	}}
	gateway := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "gw",
			"namespace": "default",
		},
	}}
	gvrServer := schema.GroupVersionResource{Group: "policy.linkerd.io", Version: "v1beta1", Resource: "servers"}
	gvrRoute := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
	gvrGateway := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gvrVirtualService := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrDestinationRule := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrServer:          "ServerList",
		gvrRoute:           "HTTPRouteList",
		gvrGateway:         "GatewayList",
		gvrVirtualService:  "VirtualServiceList",
		gvrDestinationRule: "DestinationRuleList",
	}, server, route, gateway, virtualService, virtualServiceMulti, virtualServiceOther, destinationRule)

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
				{Name: "gateways", Kind: "Gateway", Namespaced: true},
			},
		},
		{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
				{Name: "destinationrules", Kind: "DestinationRule", Namespaced: true},
			},
		},
	}
	discoveryClient := &linkerdDiscoveryResources{
		resources: resources,
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "linkerd.io"},
			{Name: "policy.linkerd.io"},
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

func TestLinkerdHealthAndProxyStatus(t *testing.T) {
	toolset := newLinkerdToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	result, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default"},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["evidence"] == nil {
		t.Fatalf("expected proxy evidence")
	}
}

func TestLinkerdMeshKindStatus(t *testing.T) {
	toolset := newLinkerdToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleVirtualServiceStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleVirtualServiceStatus: %v", err)
	}
	if _, err := toolset.handleDestinationRuleStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleDestinationRuleStatus: %v", err)
	}
}

func TestLinkerdAllowedNamespacesAndVersion(t *testing.T) {
	toolset := newLinkerdToolset(t)
	namespaces, err := toolset.allowedNamespaces(context.Background(), policy.User{Role: policy.RoleCluster}, "")
	if err != nil {
		t.Fatalf("allowedNamespaces: %v", err)
	}
	if len(namespaces) < 2 {
		t.Fatalf("expected namespaces, got %v", namespaces)
	}
	if toolset.Version() == "" {
		t.Fatalf("expected Version string")
	}
}

func TestLinkerdGatewayAndCRStatusErrors(t *testing.T) {
	toolset := newLinkerdToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleGatewayStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleGatewayStatus: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "VirtualService", "name": "multi"},
	}); err == nil {
		t.Fatalf("expected multiple namespace error")
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "Server", "name": "missing", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus not found: %v", err)
	}
}

func TestLinkerdNotDetectedPaths(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	discoveryClient := &linkerdDiscoveryResources{groups: &metav1.APIGroupList{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleIdentityIssues not detected: %v", err)
	}

	discoveryClient.groups = &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}}
	toolsetPolicy := New()
	_ = toolsetPolicy.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolsetPolicy.handlePolicyDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handlePolicyDebug missing policy group: %v", err)
	}
}

func TestLinkerdInitError(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected init error for missing clients")
	}
}

func TestLinkerdCRStatusClusterScoped(t *testing.T) {
	clusterObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "linkerd.io/v1alpha2",
		"kind":       "ClusterConfig",
		"metadata":   map[string]any{"name": "cluster"},
	}}
	gvr := schema.GroupVersionResource{Group: "linkerd.io", Version: "v1alpha2", Resource: "clusterconfigs"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "ClusterConfigList",
	}, clusterObj)
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{{
			GroupVersion: "linkerd.io/v1alpha2",
			APIResources: []metav1.APIResource{{Name: "clusterconfigs", Kind: "ClusterConfig", Namespaced: false}},
		}},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	client := k8sfake.NewSimpleClientset()
	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{Typed: client, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
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
		Arguments: map[string]any{"kind": "ClusterConfig", "name": "cluster"},
	}); err != nil {
		t.Fatalf("handleCRStatus cluster scoped: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "ClusterConfig"},
	}); err != nil {
		t.Fatalf("handleCRStatus cluster list: %v", err)
	}
}

func TestLinkerdHelperCoverage(t *testing.T) {
	pod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "linkerd-proxy"}}}}
	if !hasLinkerdProxy(pod) {
		t.Fatalf("expected linkerd-proxy")
	}
	plainPod := &corev1.Pod{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	if hasLinkerdProxy(plainPod) {
		t.Fatalf("expected no proxy container")
	}
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	if !isPodReady(pod) {
		t.Fatalf("expected ready pod")
	}
	if isPodReady(&corev1.Pod{}) {
		t.Fatalf("expected pod not ready")
	}
	if sliceIf("") != nil {
		t.Fatalf("expected nil slice for empty value")
	}
	if _, err := toUnstructured(pod); err != nil {
		t.Fatalf("toUnstructured: %v", err)
	}
}

func TestLinkerdCRStatusAdditionalBranches(t *testing.T) {
	toolset := newLinkerdToolset(t)
	clusterUser := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus list all: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      clusterUser,
		Arguments: map[string]any{"kind": "DestinationRule", "name": "api"},
	}); err != nil {
		t.Fatalf("handleCRStatus single namespace lookup: %v", err)
	}
	namespaceUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      namespaceUser,
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus namespace role: %v", err)
	}
}

func TestLinkerdIdentityNotFound(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	discoveryClient := &linkerdDiscoveryResources{groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	if _, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err != nil {
		t.Fatalf("handleIdentityIssues no deployment: %v", err)
	}
}

func TestDetectLinkerdNamespaceFallback(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}})
	discoveryClient := &linkerdDiscoveryResources{groups: &metav1.APIGroupList{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	detected, namespaces, _, err := toolset.detectLinkerd(context.Background())
	if err != nil {
		t.Fatalf("detectLinkerd: %v", err)
	}
	if !detected || len(namespaces) == 0 {
		t.Fatalf("expected namespace fallback detection")
	}
}
