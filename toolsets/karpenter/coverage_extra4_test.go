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

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestAddAWSNodeClassEvidenceMissingTools(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	toolset := New()
	ctx := mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Registry: reg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}
	ctx.Invoker = mcp.NewToolInvoker(reg, mcp.ToolContext(ctx))
	_ = toolset.Init(ctx)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
		"spec": map[string]any{
			"role":            "arn:aws:iam::123456789012:role/NodeRole",
			"instanceProfile": "arn:aws:iam::123456789012:instance-profile/NodeProfile",
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
			"securityGroupSelectorTerms": []any{
				map[string]any{"ids": []any{"sg-1"}},
			},
		},
	}}
	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	analysis := render.NewAnalysis()
	toolset.addAWSNodeClassEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, match, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected aws evidence for missing tools")
	}
}

func TestExtractSelectorInfoFromSelectorField(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"subnetSelector": map[string]any{
				"karpenter.sh/discovery": "cluster",
			},
		},
	}}
	selectors := extractAWSNodeClassSelectors(obj)
	if len(selectors.subnetTagTerms) == 0 {
		t.Fatalf("expected subnet tag terms")
	}
	if awsNameFromARN("arn:aws:iam::123456789012:role", ":role/") == "" {
		t.Fatalf("expected arn name fallback")
	}
}

func TestHandleNodePoolDebugWithConditions(t *testing.T) {
	nodePool := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "NodePool",
		"metadata": map[string]any{
			"name": "pool",
		},
		"spec": map[string]any{
			"nodeClassRef": map[string]any{"name": "default", "kind": "EC2NodeClass"},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Pending"},
			},
		},
	}}
	nodeClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
	}}
	gvrPool := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodepools"}
	gvrClass := schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1beta1", Resource: "ec2nodeclasses"}
	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrPool:  "NodePoolList",
		gvrClass: "EC2NodeClassList",
	}, nodePool, nodeClass)
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.sh/v1beta1", APIResources: []metav1.APIResource{{Name: "nodepools", Kind: "NodePool", Namespaced: false}}},
			{GroupVersion: "karpenter.k8s.aws/v1beta1", APIResources: []metav1.APIResource{{Name: "ec2nodeclasses", Kind: "EC2NodeClass", Namespaced: false}}},
		},
	}
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)
	if _, err := toolset.handleNodePoolDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleNodePoolDebug conditions: %v", err)
	}
}

func TestHandleInterruptionDebugNoResources(t *testing.T) {
	discovery := &fakeCachedDiscovery{
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "karpenter.sh/v1beta1", APIResources: []metav1.APIResource{}},
		},
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)
	if _, err := toolset.handleInterruptionDebug(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	}); err != nil {
		t.Fatalf("handleInterruptionDebug no resources: %v", err)
	}
}

func TestListResourceObjectsClusterNameFilter(t *testing.T) {
	objDefault := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "Provisioner",
		"metadata": map[string]any{
			"name":      "prov",
			"namespace": "default",
		},
	}}
	objOther := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "Provisioner",
		"metadata": map[string]any{
			"name":      "other",
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
	typed := kubefake.NewSimpleClientset()
	toolset := newMinimalKarpenterToolset(t, discovery, dyn, typed)
	match := resourceMatch{GVR: gvr, Kind: "Provisioner", Namespaced: true}
	items, _, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, match, "", "prov", "")
	if err != nil || len(items) != 1 {
		t.Fatalf("expected filtered resource, err=%v items=%d", err, len(items))
	}
}

func TestToUnstructuredNil(t *testing.T) {
	if _, err := toUnstructured(nil); err == nil {
		t.Fatalf("expected toUnstructured nil error")
	}
}
