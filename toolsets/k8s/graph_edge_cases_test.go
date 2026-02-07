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
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestGraphEdgeCases(t *testing.T) {
	namespace := "default"
	labels := map[string]string{"app": "web"}

	serviceNoSelector := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "no-selector", Namespace: namespace},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
	}
	ingressEmpty := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: namespace},
	}
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	orphanRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-rs",
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "StatefulSet",
				Name: "other",
			}},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: namespace, Labels: labels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: labels},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{
					{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/24"}},
					{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}}},
					{PodSelector: &metav1.LabelSelector{MatchLabels: labels}},
				},
			}},
		},
	}
	nsDefault := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"team": "a"}}}
	nsOther := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{"team": "a"}}}

	client := k8sfake.NewSimpleClientset(serviceNoSelector, ingressEmpty, deploy, orphanRS, pod, netpol, nsDefault, nsOther)
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

	graph := newGraphBuilder()
	if _, err := toolset.addIngressGraph(context.Background(), graph, namespace, "empty", nil); err != nil {
		t.Fatalf("addIngressGraph: %v", err)
	}
	if _, err := toolset.addServiceGraph(context.Background(), graph, namespace, "no-selector", nil); err != nil {
		t.Fatalf("addServiceGraph: %v", err)
	}
	if _, err := toolset.addDeploymentGraph(context.Background(), graph, namespace, "web", nil); err != nil {
		t.Fatalf("addDeploymentGraph: %v", err)
	}
	cache := &graphCache{namespacesLoaded: true, namespaceList: []*corev1.Namespace{nsDefault, nsOther}}
	_ = toolset.addNetworkPolicyGraph(context.Background(), graph, namespace, cache)
}
