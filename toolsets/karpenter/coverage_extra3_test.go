package karpenter

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestKarpenterCRStatusBranchesExtra(t *testing.T) {
	toolset := newKarpenterToolset(t)
	userCluster := policy.User{Role: policy.RoleCluster}

	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "NodePool", "name": "pool"},
	}); err != nil {
		t.Fatalf("handleCRStatus nodepool: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "NodePool", "name": "missing"},
	}); err != nil {
		t.Fatalf("handleCRStatus nodepool missing: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "Provisioner", "namespace": "default"},
	}); err != nil {
		t.Fatalf("handleCRStatus provisioner namespace: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      userCluster,
		Arguments: map[string]any{"kind": "Provisioner"},
	}); err != nil {
		t.Fatalf("handleCRStatus provisioner list: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}},
		Arguments: map[string]any{"kind": "Provisioner"},
	}); err != nil {
		t.Fatalf("handleCRStatus provisioner allowed list: %v", err)
	}
}

func TestHandleNodeClassDebugWithConditions(t *testing.T) {
	nodeClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Pending"},
			},
		},
	}}
	gvr := schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1beta1", Resource: "ec2nodeclasses"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "EC2NodeClassList",
	}, nodeClass)
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.k8s.aws/v1beta1", APIResources: []metav1.APIResource{{Name: "ec2nodeclasses", Kind: "EC2NodeClass", Namespaced: false}}},
		},
	}
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)
	if _, err := toolset.handleNodeClassDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleNodeClassDebug conditions: %v", err)
	}
}
