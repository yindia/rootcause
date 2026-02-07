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

func TestHandleGraphKinds(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "api"},
			Ports:    []corev1.ServicePort{{Port: 80}},
		},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Subsets: []corev1.EndpointSubset{
			{Addresses: []corev1.EndpointAddress{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1"}}}},
		},
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { v := int32(2); return &v }(),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	replicaSet := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-rs",
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "api"},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: func() *int32 { v := int32(2); return &v }(),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
	}
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
		},
	}
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: namespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-1",
			Namespace: namespace,
			Labels:    map[string]string{"app": "api"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "api-rs"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	dbPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-0",
			Namespace: namespace,
			Labels:    map[string]string{"app": "db"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "StatefulSet", Name: "db"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	agentPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-1",
			Namespace: namespace,
			Labels:    map[string]string{"app": "agent"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "agent"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "api-ing", Namespace: namespace},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "api",
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}

	client := k8sfake.NewSimpleClientset(service, endpoints, deployment, replicaSet, statefulSet, daemonSet, pod, dbPod, agentPod, ingress, netpol, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})

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
			"rules": []any{map[string]any{"backendRefs": []any{map[string]any{"name": "api", "kind": "Service"}}}},
		},
	}}
	virtualService := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata":   map[string]any{"name": "vs", "namespace": namespace},
		"spec": map[string]any{
			"hosts":    []any{"api.default.svc.cluster.local"},
			"gateways": []any{"mesh", "gw"},
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
		gvrGateway:    "GatewayList",
		gvrRoute:      "HTTPRouteList",
		gvrVS:         "VirtualServiceList",
		gvrDR:         "DestinationRuleList",
		gvrAuthz:      "AuthorizationPolicyList",
		gvrProfile:    "ServiceProfileList",
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

	user := policy.User{Role: policy.RoleCluster}
	for _, kind := range []string{"service", "deployment", "replicaset", "statefulset", "daemonset", "pod", "ingress"} {
		name := "api"
		switch kind {
		case "replicaset":
			name = "api-rs"
		case "statefulset":
			name = "db"
		case "daemonset":
			name = "agent"
		case "pod":
			name = "api-1"
		case "ingress":
			name = "api-ing"
		}
		if _, err := toolset.handleGraph(context.Background(), mcp.ToolRequest{
			User:      user,
			Arguments: map[string]any{"kind": kind, "name": name, "namespace": namespace},
		}); err != nil {
			t.Fatalf("handleGraph %s: %v", kind, err)
		}
	}
}
