package istio

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newIstioToolsetForLists(t *testing.T) *Toolset {
	t.Helper()
	vsDefault := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-default",
			"namespace": "default",
		},
	}}
	vsOther := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-other",
			"namespace": "other",
		},
	}}
	clusterObj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "install.istio.io/v1alpha1",
		"kind":       "IstioOperator",
		"metadata": map[string]any{
			"name": "demo",
		},
	}}
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrCluster := schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS:      "VirtualServiceList",
		gvrCluster: "IstioOperatorList",
	}, vsDefault, vsOther, clusterObj)

	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-default", Namespace: "default"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-other", Namespace: "other"}},
	)

	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "networking.istio.io/v1beta1",
				APIResources: []metav1.APIResource{{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}},
			},
			{
				GroupVersion: "install.istio.io/v1alpha1",
				APIResources: []metav1.APIResource{{Name: "istiooperators", Kind: "IstioOperator", Namespaced: false}},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}

	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{
		Typed:     client,
		Dynamic:   dynamicClient,
		Discovery: discoveryClient,
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

func TestHandleServiceMeshHosts(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleServiceMeshHosts(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleServiceMeshHosts: %v", err)
	}
}

func TestHandleDiscoverNamespacesEmpty(t *testing.T) {
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
	discoveryClient := &istioDiscoveryResources{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})

	_, err := toolset.handleDiscoverNamespaces(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{}},
	})
	if err != nil {
		t.Fatalf("handleDiscoverNamespaces empty: %v", err)
	}
}

func TestIstioListObjectsAndServicesBranches(t *testing.T) {
	toolset := newIstioToolsetForLists(t)
	ctx := context.Background()
	userCluster := policy.User{Role: policy.RoleCluster}
	userNamespace := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default", "other"}}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrCluster := schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}

	items, namespaces, err := toolset.listObjects(ctx, userCluster, gvr, true, "default", "")
	if err != nil || len(items) != 1 || len(namespaces) != 1 {
		t.Fatalf("listObjects namespace: items=%d namespaces=%v err=%v", len(items), namespaces, err)
	}
	items, namespaces, err = toolset.listObjects(ctx, userCluster, gvr, true, "", "")
	if err != nil || len(items) != 2 || len(namespaces) != 0 {
		t.Fatalf("listObjects cluster: items=%d namespaces=%v err=%v", len(items), namespaces, err)
	}
	items, namespaces, err = toolset.listObjects(ctx, userNamespace, gvr, true, "", "")
	if err != nil || len(items) != 2 || len(namespaces) != 2 {
		t.Fatalf("listObjects namespace-role: items=%d namespaces=%v err=%v", len(items), namespaces, err)
	}
	clusterItems, namespaces, err := toolset.listObjects(ctx, userCluster, gvrCluster, false, "", "")
	if err != nil || len(clusterItems) != 1 || len(namespaces) != 0 {
		t.Fatalf("listObjects cluster-scope: items=%d namespaces=%v err=%v", len(clusterItems), namespaces, err)
	}

	services, namespaces, err := toolset.listServices(ctx, userCluster, "")
	if err != nil || len(services) != 2 || len(namespaces) != 0 {
		t.Fatalf("listServices cluster: services=%d namespaces=%v err=%v", len(services), namespaces, err)
	}
	services, namespaces, err = toolset.listServices(ctx, userNamespace, "")
	if err != nil || len(services) != 2 || len(namespaces) != 2 {
		t.Fatalf("listServices namespace-role: services=%d namespaces=%v err=%v", len(services), namespaces, err)
	}
	services, namespaces, err = toolset.listServices(ctx, userCluster, "default")
	if err != nil || len(services) != 1 || len(namespaces) != 1 {
		t.Fatalf("listServices namespace: services=%d namespaces=%v err=%v", len(services), namespaces, err)
	}
}

func TestIstioHelperBranches(t *testing.T) {
	if matchServiceEntry("api.example.com", []string{}) {
		t.Fatalf("expected no match with empty patterns")
	}
	if !matchServiceEntry("api.example.com", []string{"api.example.com"}) {
		t.Fatalf("expected exact match")
	}
	obj := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"hosts": []any{"api.example.com"}}}}
	objWithValue := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"host": "api.example.com", "hosts": []any{"api.example.com"}}}}
	if nestedString(obj, "spec", "missing") != "" {
		t.Fatalf("expected empty nested string")
	}
	if nestedString(nil, "spec", "host") != "" {
		t.Fatalf("expected empty nested string for nil obj")
	}
	if nestedString(objWithValue, "spec", "host") != "api.example.com" {
		t.Fatalf("expected nested string value")
	}
	if len(nestedStringSlice(obj, "spec", "missing")) != 0 {
		t.Fatalf("expected empty nested slice")
	}
	if len(nestedStringSlice(nil, "spec", "hosts")) != 0 {
		t.Fatalf("expected empty nested slice for nil obj")
	}
	if len(nestedStringSlice(objWithValue, "spec", "hosts")) != 1 {
		t.Fatalf("expected nested slice value")
	}
	if _, err := toUnstructured(nil); err == nil {
		t.Fatalf("expected error for nil pod")
	}
	if toString(123) != "123" {
		t.Fatalf("expected toString int conversion")
	}
}

func TestHandleServiceMeshHostsNotDetected(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	discoveryClient := &istioDiscoveryResources{groups: &metav1.APIGroupList{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discoveryClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})
	if _, err := toolset.handleServiceMeshHosts(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleServiceMeshHosts not detected: %v", err)
	}
}

func TestHandleServiceMeshHostsNoHosts(t *testing.T) {
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
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
	if _, err := toolset.handleServiceMeshHosts(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleServiceMeshHosts no hosts: %v", err)
	}
}

func TestHandleExternalDependencyCheckMissingHosts(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleExternalDependencyCheck(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleExternalDependencyCheck missing hosts: %v", err)
	}
}

func TestHandleExternalDependencyCheckAllCovered(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
	}
	vs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-ext",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hosts": []any{"external.example.com"},
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
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrSE := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "serviceentries"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS: "VirtualServiceList",
		gvrSE: "ServiceEntryList",
	}, vs, se)
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		service,
	)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "networking.istio.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
					{Name: "serviceentries", Kind: "ServiceEntry", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
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

	if _, err := toolset.handleExternalDependencyCheck(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleExternalDependencyCheck all covered: %v", err)
	}
}

func TestHandleDiscoverNamespacesWithNamespace(t *testing.T) {
	toolset := newIstioToolset(t)
	if _, err := toolset.handleDiscoverNamespaces(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleDiscoverNamespaces namespace: %v", err)
	}
}

func TestHandleDiscoverNamespacesNamespaceDenied(t *testing.T) {
	toolset := newIstioToolset(t)
	_, err := toolset.handleDiscoverNamespaces(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}},
		Arguments: map[string]any{"namespace": "other"},
	})
	if err == nil {
		t.Fatalf("expected namespace denial error")
	}
}

func TestHandleHealthFallback(t *testing.T) {
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}},
	)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &istioDiscovery{groups: []string{"networking.istio.io"}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})
	if _, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleHealth fallback: %v", err)
	}
}

func TestHandleExternalDependencyCheckBranchCoverage(t *testing.T) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
	}
	vs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-mesh",
			"namespace": "default",
		},
		"spec": map[string]any{
			"hosts": []any{"mesh"},
		},
	}}
	dr := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "DestinationRule",
		"metadata": map[string]any{
			"name":      "dr-empty",
			"namespace": "default",
		},
		"spec": map[string]any{},
	}}
	gw := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "Gateway",
		"metadata": map[string]any{
			"name":      "gw",
			"namespace": "default",
		},
		"spec": map[string]any{
			"servers": []any{map[string]any{"hosts": []any{"api.default.svc.cluster.local"}}},
		},
	}}
	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrDR := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	gvrGW := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "gateways"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS: "VirtualServiceList",
		gvrDR: "DestinationRuleList",
		gvrGW: "GatewayList",
	}, vs, dr, gw)
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		service,
	)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "networking.istio.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
					{Name: "destinationrules", Kind: "DestinationRule", Namespaced: true},
					{Name: "gateways", Kind: "Gateway", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
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
	if _, err := toolset.handleExternalDependencyCheck(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err != nil {
		t.Fatalf("handleExternalDependencyCheck branch coverage: %v", err)
	}
}

func TestHandleCRStatusClusterScopeGatewayClass(t *testing.T) {
	gatewayClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GatewayClass",
		"metadata": map[string]any{
			"name": "demo",
		},
	}}
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "GatewayClassList",
	}, gatewayClass)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{{Name: "gatewayclasses", Kind: "GatewayClass", Namespaced: false}}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset()
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
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "GatewayClass", "name": "demo"},
	}); err != nil {
		t.Fatalf("handleCRStatus cluster-scope: %v", err)
	}
}

func TestHandleCRStatusClusterScopeList(t *testing.T) {
	gatewayClassOne := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GatewayClass",
		"metadata": map[string]any{
			"name": "one",
		},
	}}
	gatewayClassTwo := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GatewayClass",
		"metadata": map[string]any{
			"name": "two",
		},
	}}
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "GatewayClassList",
	}, gatewayClassOne, gatewayClassTwo)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{{Name: "gatewayclasses", Kind: "GatewayClass", Namespaced: false}}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset()
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
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "GatewayClass"},
	}); err != nil {
		t.Fatalf("handleCRStatus cluster-scope list: %v", err)
	}
}

func TestHandleCRStatusListNamespaceWithStatus(t *testing.T) {
	vs := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs",
			"namespace": "default",
		},
		"status": map[string]any{
			"state": "ok",
		},
	}}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "VirtualServiceList",
	}, vs)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "networking.istio.io/v1beta1", APIResources: []metav1.APIResource{{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
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
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "VirtualService", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus list namespace: %v", err)
	}
}

func TestHandleCRStatusBranches(t *testing.T) {
	vsDefault := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs",
			"namespace": "default",
			"labels":    map[string]any{"env": "dev"},
		},
	}}
	vsDefaultOnly := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs-single",
			"namespace": "default",
		},
	}}
	vsOther := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]any{
			"name":      "vs",
			"namespace": "other",
		},
	}}
	gatewayClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "GatewayClass",
		"metadata": map[string]any{
			"name": "gc1",
		},
	}}

	gvrVS := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	gvrGC := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrVS: "VirtualServiceList",
		gvrGC: "GatewayClassList",
	}, vsDefault, vsDefaultOnly, vsOther, gatewayClass)
	discoveryClient := &istioDiscoveryResources{
		resources: []*metav1.APIResourceList{
			{GroupVersion: "networking.istio.io/v1beta1", APIResources: []metav1.APIResource{{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}}},
			{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{{Name: "gatewayclasses", Kind: "GatewayClass", Namespaced: false}}},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "networking.istio.io"}}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
	)
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
	ctx := context.Background()
	userCluster := policy.User{Role: policy.RoleCluster}

	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService", "name": "vs", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus name+namespace: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService", "name": "missing", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus missing name: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService", "name": "vs"},
	}); err == nil {
		t.Fatalf("expected multiple namespace error")
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService", "name": "vs-single"},
	}); err != nil {
		t.Fatalf("handleCRStatus name search single: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService", "namespace": "default", "labelSelector": "env=dev"},
	}); err != nil {
		t.Fatalf("handleCRStatus list namespace: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus list all namespaces: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}},
		Arguments: map[string]any{"kind": "VirtualService"},
	}); err != nil {
		t.Fatalf("handleCRStatus list allowed namespaces: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "GatewayClass", "name": "gc1"},
	}); err != nil {
		t.Fatalf("handleCRStatus gatewayclass: %v", err)
	}
	if _, err := toolset.handleCRStatus(ctx, mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "GatewayClass"},
	}); err != nil {
		t.Fatalf("handleCRStatus gatewayclass list: %v", err)
	}
}

func TestListObjectsAndServicesDenied(t *testing.T) {
	toolset := newIstioToolsetForLists(t)
	user := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	gvr := schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	if _, _, err := toolset.listObjects(context.Background(), user, gvr, true, "other", ""); err == nil {
		t.Fatalf("expected listObjects namespace denial")
	}
	if _, _, err := toolset.listServices(context.Background(), user, "other"); err == nil {
		t.Fatalf("expected listServices namespace denial")
	}
	gvrCluster := schema.GroupVersionResource{Group: "install.istio.io", Version: "v1alpha1", Resource: "istiooperators"}
	if _, _, err := toolset.listObjects(context.Background(), user, gvrCluster, false, "", ""); err == nil {
		t.Fatalf("expected listObjects cluster-scope denial")
	}
}

func TestDetectIstioMissingDiscovery(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: k8sfake.NewSimpleClientset()},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: k8sfake.NewSimpleClientset()}),
	})
	if _, _, err := toolset.detectIstio(context.Background()); err == nil {
		t.Fatalf("expected detectIstio error")
	}
}
