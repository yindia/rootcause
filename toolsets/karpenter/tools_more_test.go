package karpenter

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newKarpenterToolset(t *testing.T) *Toolset {
	t.Helper()
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karpenter",
			Namespace: "karpenter",
			Labels: map[string]string{
				karpenterSelector: "controller",
			},
		},
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 0},
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}

	client := k8sfake.NewSimpleClientset(
		deploy,
		node,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "karpenter"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)

	nodePool := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "NodePool",
		"metadata": map[string]any{
			"name": "pool",
		},
		"spec": map[string]any{
			"requirements": []any{
				map[string]any{"key": "instance-type", "operator": "In", "values": []any{"m5.large"}},
			},
			"nodeClassRef":         map[string]any{"name": "default", "kind": "EC2NodeClass"},
			"limits":               map[string]any{"cpu": "1000"},
			"weight":               int64(5),
			"ttlSecondsAfterEmpty": int64(30),
		},
	}}
	provisioner := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "Provisioner",
		"metadata": map[string]any{
			"name":      "prov",
			"namespace": "default",
		},
		"spec": map[string]any{
			"requirements": []any{
				map[string]any{"key": "capacity-type", "operator": "In", "values": []any{"spot"}},
			},
		},
	}}
	nodeClaim := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.sh/v1beta1",
		"kind":       "NodeClaim",
		"metadata": map[string]any{
			"name": "claim",
		},
		"spec": map[string]any{
			"nodeName":     "node-1",
			"nodeClassRef": map[string]any{"name": "default", "kind": "EC2NodeClass"},
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "False", "reason": "Init"},
				map[string]any{"type": "Drifted", "status": "True", "reason": "Drift"},
			},
		},
	}}
	nodeClass := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "karpenter.k8s.aws/v1beta1",
		"kind":       "EC2NodeClass",
		"metadata": map[string]any{
			"name": "default",
		},
		"spec": map[string]any{
			"role": "karpenter-role",
			"subnetSelectorTerms": []any{
				map[string]any{"ids": []any{"subnet-1"}},
			},
			"securityGroupSelectorTerms": []any{
				map[string]any{"ids": []any{"sg-1"}},
			},
		},
	}}

	gvrPool := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodepools"}
	gvrProvisioner := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}
	gvrClaim := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodeclaims"}
	gvrClass := schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1beta1", Resource: "ec2nodeclasses"}

	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrPool:        "NodePoolList",
		gvrProvisioner: "ProvisionerList",
		gvrClaim:       "NodeClaimList",
		gvrClass:       "EC2NodeClassList",
	}, nodePool, provisioner, nodeClaim, nodeClass)

	discovery := &fakeCachedDiscovery{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "karpenter.sh/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "nodepools", Kind: "NodePool", Namespaced: false},
					{Name: "provisioners", Kind: "Provisioner", Namespaced: true},
					{Name: "nodeclaims", Kind: "NodeClaim", Namespaced: false},
				},
			},
			{
				GroupVersion: "karpenter.k8s.aws/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "ec2nodeclasses", Kind: "EC2NodeClass", Namespaced: false},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{
			{Name: "karpenter.sh"},
			{Name: "karpenter.k8s.aws"},
		}},
	}
	groupResources, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		t.Fatalf("get api group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	cfg := config.DefaultConfig()
	toolset := New()
	clients := &kube.Clients{
		Typed:     client,
		Dynamic:   dynamicClient,
		Discovery: discovery,
		Mapper:    mapper,
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

func TestKarpenterStatusAndCRStatus(t *testing.T) {
	toolset := newKarpenterToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      user,
		Arguments: map[string]any{"kind": "NodePool", "name": "pool"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
}

func TestKarpenterNodePoolNodeClassInterruption(t *testing.T) {
	toolset := newKarpenterToolset(t)
	user := policy.User{Role: policy.RoleCluster}
	if _, err := toolset.handleNodePoolDebug(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleNodePoolDebug: %v", err)
	}
	if _, err := toolset.handleNodeClassDebug(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleNodeClassDebug: %v", err)
	}
	if _, err := toolset.handleInterruptionDebug(context.Background(), mcp.ToolRequest{User: user}); err != nil {
		t.Fatalf("handleInterruptionDebug: %v", err)
	}
}

func TestKarpenterListResourceObjectsAndHelpers(t *testing.T) {
	toolset := newKarpenterToolset(t)
	matchNamespaced := resourceMatch{
		GVR:        schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"},
		Kind:       "Provisioner",
		Namespaced: true,
	}
	items, namespaces, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "default", "prov", "")
	if err != nil {
		t.Fatalf("listResourceObjects: %v", err)
	}
	if len(items) != 1 || len(namespaces) != 1 {
		t.Fatalf("expected provisioner results")
	}

	matchCluster := resourceMatch{
		GVR:        schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodepools"},
		Kind:       "NodePool",
		Namespaced: false,
	}
	clusterItems, _, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchCluster, "", "pool", "")
	if err != nil {
		t.Fatalf("listResourceObjects cluster: %v", err)
	}
	if len(clusterItems) != 1 {
		t.Fatalf("expected nodepool results")
	}

	namespaces, err = toolset.allowedNamespaces(context.Background(), policy.User{Role: policy.RoleCluster}, "")
	if err != nil {
		t.Fatalf("allowedNamespaces cluster: %v", err)
	}
	if len(namespaces) < 2 {
		t.Fatalf("expected namespace list")
	}
	nsUser := policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}
	nsList, err := toolset.allowedNamespaces(context.Background(), nsUser, "")
	if err != nil || len(nsList) != 1 {
		t.Fatalf("allowedNamespaces namespace: %v", err)
	}

	if !podInList("default", "pod", []string{"pod", "default/pod"}) {
		t.Fatalf("expected podInList to match")
	}
	if podInList("default", "pod", []string{"other"}) {
		t.Fatalf("expected podInList to miss")
	}
	reason, _ := pendingReason(&corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}})
	if reason == "" {
		t.Fatalf("expected pending reason")
	}
	if extractDisruption(&unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"ttlSecondsAfterEmpty": int64(10)}}}) == nil {
		t.Fatalf("expected disruption from ttl")
	}
	if nestedInt(&unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"weight": int64(3)}}}, "spec", "weight") == 0 {
		t.Fatalf("expected nestedInt value")
	}
}
