package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestAddStatefulSetAndDaemonSetGraph(t *testing.T) {
	namespace := "default"
	replicas := int32(1)
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: "db",
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "db"}},
			},
		},
		Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: namespace},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "agent"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "agent"}},
			},
		},
		Status: appsv1.DaemonSetStatus{NumberReady: 1, DesiredNumberScheduled: 1},
	}
	ssPod := &corev1.Pod{
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
	dsPod := &corev1.Pod{
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
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "db"}},
	}
	endpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "db-0"}},
				},
			},
		},
	}

	client := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}},
		ss, ds, ssPod, dsPod, service, endpoints,
	)
	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{Typed: client}
	if err := toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(clients),
	}); err != nil {
		t.Fatalf("init: %v", err)
	}

	graph := newGraphBuilder()
	cache, warnings := toolset.buildGraphCache(context.Background(), namespace, true)
	if warnings == nil {
		t.Fatalf("expected warnings slice")
	}
	if _, err := toolset.addStatefulSetGraph(context.Background(), graph, namespace, "db", cache); err != nil {
		t.Fatalf("add statefulset graph: %v", err)
	}
	if _, err := toolset.addDaemonSetGraph(context.Background(), graph, namespace, "agent", cache); err != nil {
		t.Fatalf("add daemonset graph: %v", err)
	}
	if len(graph.nodes) == 0 || len(graph.edges) == 0 {
		t.Fatalf("expected graph nodes and edges")
	}
}
