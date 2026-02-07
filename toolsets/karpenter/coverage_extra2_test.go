package karpenter

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newMinimalKarpenterToolset(t *testing.T, discovery *fakeCachedDiscovery, dyn *dynamicfake.FakeDynamicClient, typed *kubefake.Clientset) *Toolset {
	t.Helper()
	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{
		Typed:     typed,
		Dynamic:   dyn,
		Discovery: discovery,
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

func TestHandleNodePoolDebugNoResources(t *testing.T) {
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.sh/v1beta1", APIResources: []metav1.APIResource{}},
		},
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)

	if _, err := toolset.handleNodePoolDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleNodePoolDebug no resources: %v", err)
	}
}

func TestHandleNodeClassDebugNoResources(t *testing.T) {
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.k8s.aws/v1beta1", APIResources: []metav1.APIResource{}},
		},
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)

	if _, err := toolset.handleNodeClassDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleNodeClassDebug no resources: %v", err)
	}
}

func TestHandleInterruptionDebugNodeEvidence(t *testing.T) {
	nodeClaim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "NodeClaim",
		"metadata": map[string]any{
			"name": "claim",
		},
		"spec": map[string]any{
			"nodeClassRef": map[string]any{"name": "default"},
		},
		"status": map[string]any{
			"nodeName": "node-1",
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Init"},
				map[string]any{"type": "Drifted", "status": "True", "reason": "Drift"},
			},
		},
	}}
	gvrClaim := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodeclaims"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrClaim: "NodeClaimList",
	}, nodeClaim)
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.sh/v1beta1", APIResources: []metav1.APIResource{{Name: "nodeclaims", Kind: "NodeClaim", Namespaced: false}}},
		},
	}
	typed := kubefake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}})
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)

	if _, err := toolset.handleInterruptionDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleInterruptionDebug: %v", err)
	}
}

func TestSelectorIDsAndAWSNameFromARN(t *testing.T) {
	term := map[string]any{
		"ids":      []any{"subnet-1", "subnet-2"},
		"groupIds": []string{"sg-1"},
		"id":       "subnet-3",
	}
	ids := selectorIDs(term, []string{"ids", "groupIds", "id"})
	if len(ids) != 4 {
		t.Fatalf("expected selector IDs, got %v", ids)
	}
	if awsNameFromARN("role-name", ":role/") != "role-name" {
		t.Fatalf("expected raw name passthrough")
	}
	if awsNameFromARN("arn:aws:iam::123456789012:role/KarpenterRole", ":role/") != "KarpenterRole" {
		t.Fatalf("expected role name from ARN")
	}
	if awsNameFromARN("arn:aws:iam::123456789012:instance-profile/path/ProfileName", ":instance-profile/") != "ProfileName" {
		t.Fatalf("expected instance profile name from ARN")
	}
}

func TestSelectNodeClassRefAndToString(t *testing.T) {
	withRef := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"nodeClassRef": map[string]any{"name": "default", "kind": "EC2NodeClass"}},
	}}
	ref := selectNodeClassRef(withRef)
	if ref == nil || ref["name"] != "default" {
		t.Fatalf("expected nodeClassRef")
	}
	withTemplate := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"nodeClassRef": map[string]any{"name": "templated"}}}},
	}}
	ref = selectNodeClassRef(withTemplate)
	if ref == nil || ref["name"] != "templated" {
		t.Fatalf("expected template nodeClassRef")
	}
	if toString(42) != "42" {
		t.Fatalf("expected toString to stringify non-string")
	}
}

func TestListResourceObjectsMultipleNamespaceError(t *testing.T) {
	objDefault := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "Provisioner",
		"metadata": map[string]any{
			"name":      "dup",
			"namespace": "default",
		},
	}}
	objOther := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "Provisioner",
		"metadata": map[string]any{
			"name":      "dup",
			"namespace": "other",
		},
	}}
	gvr := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "ProvisionerList",
	}, objDefault, objOther)
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.sh/v1beta1", APIResources: []metav1.APIResource{{Name: "provisioners", Kind: "Provisioner", Namespaced: true}}},
		},
	}
	typed := kubefake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
	)
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)

	match := resourceMatch{GVR: gvr, Kind: "Provisioner", Namespaced: true}
	_, _, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default", "other"}}, match, "", "dup", "")
	if err == nil {
		t.Fatalf("expected error for duplicate resource in namespaces")
	}
}
