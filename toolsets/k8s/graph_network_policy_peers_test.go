package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddNetworkPolicyPeerEdgesBranches(t *testing.T) {
	graph := newGraphBuilder()
	cache := newGraphCache()
	cache.namespacesLoaded = true
	cache.namespaceList = []*corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "default", Labels: map[string]string{"env": "prod"}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "other", Labels: map[string]string{"env": "dev"}}},
	}
	cache.podsLoaded = true
	cache.podList = []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default", Labels: map[string]string{"app": "api"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	}
	toolset := New()
	policyID := graph.addNode("NetworkPolicy", "", "default", "np", nil)
	warnings := []string{}

	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", policyID, networkingv1.NetworkPolicyPeer{
		IPBlock: &networkingv1.IPBlock{
			CIDR:   "10.0.0.0/24",
			Except: []string{"10.0.0.5/32"},
		},
	}, "allows-from", cache, &warnings)

	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", policyID, networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
	}, "allows-from", cache, &warnings)

	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", policyID, networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "bad", Operator: "Invalid"}}},
	}, "allows-from", cache, &warnings)

	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", policyID, networkingv1.NetworkPolicyPeer{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "dev"}},
		PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
	}, "allows-from", cache, &warnings)

	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", policyID, networkingv1.NetworkPolicyPeer{
		PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
	}, "allows-from", cache, &warnings)

	if len(graph.nodes) == 0 || len(graph.edges) == 0 {
		t.Fatalf("expected graph nodes and edges")
	}
}
