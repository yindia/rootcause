package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

func newGraphToolset() *Toolset {
	namespace := "default"
	labels := map[string]string{"app": "api"}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: labels, Ports: []corev1.ServicePort{{Port: 80}}},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1"}}},
		}},
	}
	rsUID := types.UID("rs-uid")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "ReplicaSet", Name: "api-rs", UID: rsUID,
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	replicas := int32(1)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-rs",
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: labels}},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "api",
			Selector:    &metav1.LabelSelector{MatchLabels: labels},
			Template:    corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: namespace},
		Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}}},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api"}}}}}}}}},
	}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny", Namespace: namespace},
		Spec:       networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchLabels: labels}},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}

	typed := k8sfake.NewSimpleClientset(svc, endpoints, pod, rs, deploy, sts, ds, ingress, netpol, ns)

	objects := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "gateway.networking.k8s.io/v1",
				"kind":       "Gateway",
				"metadata":   map[string]any{"name": "gateway", "namespace": namespace},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "gateway.networking.k8s.io/v1",
				"kind":       "HTTPRoute",
				"metadata":   map[string]any{"name": "route", "namespace": namespace},
				"spec": map[string]any{
					"parentRefs": []any{map[string]any{"name": "gateway"}},
					"rules": []any{map[string]any{"backendRefs": []any{map[string]any{"name": "api"}}}},
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "networking.istio.io/v1beta1",
				"kind":       "VirtualService",
				"metadata":   map[string]any{"name": "api", "namespace": namespace},
				"spec": map[string]any{
					"hosts":    []any{"api.default.svc.cluster.local"},
					"gateways": []any{"gateway-1"},
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "security.istio.io/v1beta1",
				"kind":       "AuthorizationPolicy",
				"metadata":   map[string]any{"name": "allow", "namespace": namespace},
				"spec": map[string]any{
					"selector": map[string]any{"matchLabels": map[string]any{"app": "api"}},
					"rules": []any{map[string]any{"from": []any{map[string]any{"source": map[string]any{"principals": []any{"spiffe://cluster.local/ns/default/sa/api"}}}}}},
				},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "linkerd.io/v1alpha2",
				"kind":       "ServiceProfile",
				"metadata":   map[string]any{"name": "api", "namespace": namespace},
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "policy.linkerd.io/v1alpha1",
				"kind":       "ServerAuthorization",
				"metadata":   map[string]any{"name": "authz", "namespace": namespace},
				"spec": map[string]any{
					"server": "api-server",
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	runtimeObjects := make([]runtime.Object, 0, len(objects))
	for _, obj := range objects {
		runtimeObjects = append(runtimeObjects, obj)
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}:     "GatewayList",
		{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}:   "HTTPRouteList",
		{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}: "VirtualServiceList",
		{Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"}: "AuthorizationPolicyList",
		{Group: "linkerd.io", Version: "v1alpha2", Resource: "serviceprofiles"}:            "ServiceProfileList",
		{Group: "policy.linkerd.io", Version: "v1alpha1", Resource: "serverauthorizations"}: "ServerAuthorizationList",
	}, runtimeObjects...)

	resources := []*metav1.APIResourceList{
		{GroupVersion: "gateway.networking.k8s.io/v1", APIResources: []metav1.APIResource{
			{Name: "gateways", Kind: "Gateway", Namespaced: true},
			{Name: "httproutes", Kind: "HTTPRoute", Namespaced: true},
		}},
		{GroupVersion: "networking.istio.io/v1beta1", APIResources: []metav1.APIResource{
			{Name: "virtualservices", Kind: "VirtualService", Namespaced: true},
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
	}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{Name: "gateway.networking.k8s.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "gateway.networking.k8s.io/v1", Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "gateway.networking.k8s.io/v1", Version: "v1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "gateways", Kind: "Gateway", Namespaced: true}, {Name: "httproutes", Kind: "HTTPRoute", Namespaced: true}},
			},
		},
		{
			Group: metav1.APIGroup{Name: "networking.istio.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "networking.istio.io/v1beta1", Version: "v1beta1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "networking.istio.io/v1beta1", Version: "v1beta1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {{Name: "virtualservices", Kind: "VirtualService", Namespaced: true}},
			},
		},
		{
			Group: metav1.APIGroup{Name: "security.istio.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "security.istio.io/v1beta1", Version: "v1beta1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "security.istio.io/v1beta1", Version: "v1beta1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {{Name: "authorizationpolicies", Kind: "AuthorizationPolicy", Namespaced: true}},
			},
		},
		{
			Group: metav1.APIGroup{Name: "linkerd.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "linkerd.io/v1alpha2", Version: "v1alpha2"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "linkerd.io/v1alpha2", Version: "v1alpha2"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1alpha2": {{Name: "serviceprofiles", Kind: "ServiceProfile", Namespaced: true}},
			},
		},
		{
			Group: metav1.APIGroup{Name: "policy.linkerd.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "policy.linkerd.io/v1alpha1", Version: "v1alpha1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "policy.linkerd.io/v1alpha1", Version: "v1alpha1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1alpha1": {{Name: "serverauthorizations", Kind: "ServerAuthorization", Namespaced: true}},
			},
		},
	})

	clients := &kube.Clients{Typed: typed, Dynamic: dyn, Discovery: cached, Mapper: mapper}
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

func TestHandleGraphService(t *testing.T) {
	toolset := newGraphToolset()
	result, err := toolset.handleGraph(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"kind":      "service",
			"name":      "api",
			"namespace": "default",
		},
	})
	if err != nil {
		t.Fatalf("handleGraph: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result")
	}
	switch nodes := data["nodes"].(type) {
	case []graphNode:
		if len(nodes) == 0 {
			t.Fatalf("expected nodes")
		}
	case []any:
		if len(nodes) == 0 {
			t.Fatalf("expected nodes")
		}
	default:
		t.Fatalf("unexpected nodes type")
	}
	switch edges := data["edges"].(type) {
	case []graphEdge:
		if len(edges) == 0 {
			t.Fatalf("expected edges")
		}
	case []any:
		if len(edges) == 0 {
			t.Fatalf("expected edges")
		}
	default:
		t.Fatalf("unexpected edges type")
	}
}

func TestHandleGraphDeploymentKinds(t *testing.T) {
	toolset := newGraphToolset()
	tests := []struct {
		kind string
		name string
	}{
		{kind: "deployment", name: "api"},
		{kind: "replicaset", name: "api-rs"},
		{kind: "statefulset", name: "db"},
		{kind: "daemonset", name: "agent"},
		{kind: "ingress", name: "api"},
	}
	for _, tt := range tests {
		_, err := toolset.handleGraph(context.Background(), mcp.ToolRequest{
			User: policy.User{Role: policy.RoleCluster},
			Arguments: map[string]any{
				"kind":      tt.kind,
				"name":      tt.name,
				"namespace": "default",
			},
		})
		if err != nil {
			t.Fatalf("handleGraph %s: %v", tt.kind, err)
		}
	}
}
