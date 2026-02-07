package karpenter

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newStatusToolset(t *testing.T, discovery *fakeCachedDiscovery, client *k8sfake.Clientset) *Toolset {
	t.Helper()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: discovery},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	return toolset
}

func TestKarpenterStatusNotDetectedExtra(t *testing.T) {
	toolset := newStatusToolset(t, &fakeCachedDiscovery{groups: &metav1.APIGroupList{}}, k8sfake.NewSimpleClientset())
	result, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence")
	}
}

func TestKarpenterStatusNamespaceFallbackExtra(t *testing.T) {
	discovery := &fakeCachedDiscovery{groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}}}
	toolset := newStatusToolset(t, discovery, k8sfake.NewSimpleClientset())
	result, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "namespaceFallback" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected namespace fallback evidence")
	}
}

func TestKarpenterStatusDeploymentListErrorExtra(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("list fail")
	})
	discovery := &fakeCachedDiscovery{groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}}}
	toolset := newStatusToolset(t, discovery, client)
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected deployment list error")
	}
}

func TestKarpenterStatusNamespaceDeniedExtra(t *testing.T) {
	discovery := &fakeCachedDiscovery{groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}}}
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "karpenter"}})
	toolset := newStatusToolset(t, discovery, client)
	_, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"other"}}})
	if err == nil {
		t.Fatalf("expected namespace denied")
	}
}
