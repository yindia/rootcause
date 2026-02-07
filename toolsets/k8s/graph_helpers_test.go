package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func TestGraphHelperFunctions(t *testing.T) {
	key := graphCacheKey("service", "default", "api", true)
	if key == "" {
		t.Fatalf("expected cache key")
	}
	meta := metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{{Kind: "Deployment", Name: "api"}}}
	if !ownedBy(&meta, "Deployment", "api") {
		t.Fatalf("expected ownedBy true")
	}
	err := &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{schema.GroupVersion{Group: "x", Version: "v1"}: nil}}
	if !discoveryErrorIsPartial(err) {
		t.Fatalf("expected partial discovery error")
	}
}

func TestAddNetworkPolicyPeerEdges(t *testing.T) {
	toolset := newGraphToolset()
	graph := newGraphBuilder()
	cache, _ := toolset.buildGraphCache(context.Background(), "default", true)
	warnings := []string{}
	peer := networkingv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}}
	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", "policy", peer, "allows-from", cache, &warnings)
	if len(graph.edges) == 0 {
		t.Fatalf("expected peer edges")
	}
	ipPeer := networkingv1.NetworkPolicyPeer{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/24"}}
	toolset.addNetworkPolicyPeerEdges(context.Background(), graph, "default", "policy", ipPeer, "allows-from", cache, &warnings)
}

func TestAddMeshGraphs(t *testing.T) {
	toolset := newGraphToolset()
	graph := newGraphBuilder()
	services := []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}}}
	index := buildServiceIndex("default", services)
	_ = toolset.addIstioGraph(context.Background(), graph, "default", index)
	_ = toolset.addLinkerdGraph(context.Background(), graph, "default", index)
	if len(graph.nodes) == 0 {
		t.Fatalf("expected mesh nodes")
	}
}
