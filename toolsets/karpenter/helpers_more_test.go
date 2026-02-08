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
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestExtendedHelpers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"requirements": []any{
				map[string]any{"key": "instance-type", "operator": "In", "values": []any{"m5.large"}, "minValues": float64(1)},
			},
			"taints": []any{
				map[string]any{"key": "dedicated", "value": "gpu", "effect": "NoSchedule"},
			},
			"limits": map[string]any{"cpu": "1000"},
			"disruption": map[string]any{"consolidationPolicy": "WhenUnderutilized"},
			"nodeClassRef": map[string]any{
				"name": "default",
				"kind": "EC2NodeClass",
			},
			"providerRef": map[string]any{"name": "provider"},
			"template": map[string]any{
				"spec": map[string]any{
					"nodeClassRef": map[string]any{"name": "templ"},
				},
			},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "NotReady"},
			},
		},
	}}
	if reqs := extractRequirements(obj); len(reqs) != 1 {
		t.Fatalf("expected requirements")
	}
	if taints := extractTaints(obj); len(taints) != 1 {
		t.Fatalf("expected taints")
	}
	if limits := extractLimits(obj); limits["cpu"] != "1000" {
		t.Fatalf("unexpected limits: %#v", limits)
	}
	if disruption := extractDisruption(obj); disruption["consolidationPolicy"] == nil {
		t.Fatalf("expected disruption data")
	}
	if ref := extractNodeClassRef(obj); ref["name"] != "default" {
		t.Fatalf("unexpected node class ref: %#v", ref)
	}
	if ref := extractNodeClassRefFromTemplate(obj); ref["name"] != "templ" {
		t.Fatalf("unexpected template node class ref: %#v", ref)
	}
	if ref := extractProviderRef(obj); ref["name"] != "provider" {
		t.Fatalf("unexpected provider ref: %#v", ref)
	}
	index := &nodeClassIndex{
		byKind: map[string]map[string]struct{}{"ec2nodeclass": {"default": {}}},
		byName: map[string][]string{"default": {"EC2NodeClass"}},
	}
	resolved := (&Toolset{}).resolveNodeClassRef(map[string]any{"name": "default", "kind": "EC2NodeClass"}, index)
	if resolved["found"] != true {
		t.Fatalf("expected node class resolution")
	}
	if out := filterByName([]unstructured.Unstructured{{Object: map[string]any{"metadata": map[string]any{"name": "a"}}}}, "a"); len(out) != 1 {
		t.Fatalf("expected filterByName to keep item")
	}
	if !isConditionFalse(map[string]any{"type": "Ready", "status": "False"}, []string{"Ready"}) {
		t.Fatalf("expected condition false")
	}
}

func TestBuildNodeClassIndex(t *testing.T) {
	nodeClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
	}}
	gvrClass := schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1beta1", Resource: "ec2nodeclasses"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrClass: "EC2NodeClassList",
	}, nodeClass)
	discoveryClient := &fakeCachedDiscovery{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "karpenter.k8s.aws/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "ec2nodeclasses", Kind: "EC2NodeClass", Namespaced: false},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.k8s.aws"}}},
	}

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dynamicClient, Discovery: discoveryClient},
		Policy:  policy.NewAuthorizer(),
	})
	index, err := toolset.buildNodeClassIndex(context.Background(), policy.User{Role: policy.RoleCluster})
	if err != nil {
		t.Fatalf("buildNodeClassIndex: %v", err)
	}
	if index == nil || len(index.byName["default"]) == 0 {
		t.Fatalf("expected node class index")
	}
}

func TestHandleNodeProvisioningDebug(t *testing.T) {
	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending",
			Namespace: "default",
			UID:       "uid-pending",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: "no nodes",
				},
			},
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: "evt", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{
			UID: pendingPod.UID,
		},
	}
	client := k8sfake.NewSimpleClientset(
		pendingPod,
		event,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
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
	result, err := toolset.handleNodeProvisioningDebug(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	})
	if err != nil {
		t.Fatalf("handleNodeProvisioningDebug: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["evidence"] == nil {
		t.Fatalf("expected evidence output")
	}
}
