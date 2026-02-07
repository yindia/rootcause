package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newGraphToolsetRich() *Toolset {
	labelsAPI := map[string]string{"app": "api"}
	labelsDB := map[string]string{"app": "db"}
	labelsAgent := map[string]string{"app": "agent"}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: labelsAPI, Ports: []corev1.ServicePort{{Port: 80}}},
	}
	headless := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "headless", Namespace: "default"},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "api-1"}}},
		}},
	}
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labelsAPI},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labelsAPI}},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-rs",
			Namespace: "default",
			Labels:    labelsAPI,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment", Name: "api",
			}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: labelsAPI}},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1},
	}
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "headless",
			Selector:    &metav1.LabelSelector{MatchLabels: labelsDB},
			Template:    corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labelsDB}},
		},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec:       appsv1.DaemonSetSpec{Selector: &metav1.LabelSelector{MatchLabels: labelsAgent}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labelsAgent}}},
		Status:     appsv1.DaemonSetStatus{NumberReady: 1, DesiredNumberScheduled: 1},
	}
	podAPI := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "api-1",
			Namespace:       "default",
			Labels:          labelsAPI,
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "api-rs"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	podDB := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "db-0",
			Namespace:       "default",
			Labels:          labelsDB,
			OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	podAgent := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "agent-1",
			Namespace:       "default",
			Labels:          labelsAgent,
			OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "agent"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "ing", Namespace: "default"},
		Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{
				{Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "api"}}},
				{Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "missing"}}},
			}}},
		}}},
	}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny", Namespace: "default"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: labelsAPI},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "b"}},
				}},
			}},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/24"}}},
			}},
		},
	}
	nsDefault := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", Labels: map[string]string{"team": "a"}}}
	nsOther := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{"team": "b"}}}

	client := k8sfake.NewSimpleClientset(service, headless, endpoints, deploy, rs, sts, ds, podAPI, podDB, podAgent, ingress, netpol, nsDefault, nsOther)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: stubCollector{},
	})
	return toolset
}

func TestGraphCoveragePaths(t *testing.T) {
	toolset := newGraphToolsetRich()
	ctx := context.Background()
	cache, _ := toolset.buildGraphCache(ctx, "default", true)
	graph := newGraphBuilder()

	if _, err := toolset.addServiceGraph(ctx, graph, "default", "api", cache); err != nil {
		t.Fatalf("addServiceGraph api: %v", err)
	}
	if _, err := toolset.addServiceGraph(ctx, graph, "default", "headless", cache); err != nil {
		t.Fatalf("addServiceGraph headless: %v", err)
	}
	if _, err := toolset.addDeploymentGraph(ctx, graph, "default", "api", cache); err != nil {
		t.Fatalf("addDeploymentGraph: %v", err)
	}
	if _, err := toolset.addStatefulSetGraph(ctx, graph, "default", "db", cache); err != nil {
		t.Fatalf("addStatefulSetGraph: %v", err)
	}
	if _, err := toolset.addDaemonSetGraph(ctx, graph, "default", "agent", cache); err != nil {
		t.Fatalf("addDaemonSetGraph: %v", err)
	}
	if _, err := toolset.addIngressGraph(ctx, graph, "default", "ing", cache); err != nil {
		t.Fatalf("addIngressGraph: %v", err)
	}
	_ = toolset.addNetworkPolicyGraph(ctx, graph, "default", cache)
	if len(graph.nodes) == 0 || len(graph.edges) == 0 {
		t.Fatalf("expected graph nodes/edges")
	}
}

func TestGraphCacheGettersMissing(t *testing.T) {
	toolset := newGraphToolsetRich()
	ctx := context.Background()
	cache, _ := toolset.buildGraphCache(ctx, "default", true)
	if _, err := toolset.getStatefulSet(ctx, cache, "default", "missing"); err == nil {
		t.Fatalf("expected missing statefulset error")
	}
	if _, err := toolset.getDaemonSet(ctx, cache, "default", "missing"); err == nil {
		t.Fatalf("expected missing daemonset error")
	}
}

func TestAddReplicaSetPodsNil(t *testing.T) {
	toolset := newGraphToolsetRich()
	graph := newGraphBuilder()
	cache := newGraphCache()
	if _, err := toolset.addReplicaSetPods(context.Background(), graph, "default", nil, cache); err != nil {
		t.Fatalf("addReplicaSetPods nil: %v", err)
	}
}
