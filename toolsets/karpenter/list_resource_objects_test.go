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
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func newListToolset(t *testing.T) *Toolset {
	t.Helper()
	provDefault := &unstructured.Unstructured{}
	provDefault.SetAPIVersion("karpenter.sh/v1beta1")
	provDefault.SetKind("Provisioner")
	provDefault.SetName("prov")
	provDefault.SetNamespace("default")
	provOther := provDefault.DeepCopy()
	provOther.SetNamespace("other")
	nodePool := &unstructured.Unstructured{}
	nodePool.SetAPIVersion("karpenter.sh/v1beta1")
	nodePool.SetKind("NodePool")
	nodePool.SetName("pool")

	gvrProv := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}
	gvrPool := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodepools"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrProv: "ProvisionerList",
		gvrPool: "NodePoolList",
	}, provDefault, provOther, nodePool)
	typed := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
	)
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Dynamic: dynamicClient, Typed: typed},
		Policy:  policy.NewAuthorizer(),
	})
	return toolset
}

func TestListResourceObjectsBranches(t *testing.T) {
	toolset := newListToolset(t)
	matchNamespaced := resourceMatch{GVR: schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}, Kind: "Provisioner", Namespaced: true}
	matchCluster := resourceMatch{GVR: schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "nodepools"}, Kind: "NodePool", Namespaced: false}

	// namespaced: namespace + name
	items, namespaces, err := toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "default", "prov", "")
	if err != nil || len(items) != 1 || len(namespaces) != 1 {
		t.Fatalf("expected namespaced get, got %v %v", err, namespaces)
	}
	// namespaced: namespace + missing name
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "default", "missing", "")
	if err != nil || len(items) != 0 || namespaces != nil {
		t.Fatalf("expected empty result for missing name")
	}
	// namespaced: namespace list
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "default", "", "")
	if err != nil || len(items) == 0 || len(namespaces) != 1 {
		t.Fatalf("expected namespace list")
	}
	// namespaced: cluster list with name filter
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "", "prov", "")
	if err != nil || len(items) == 0 || namespaces != nil {
		t.Fatalf("expected cluster name filter")
	}
	// namespaced: cluster list
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchNamespaced, "", "", "")
	if err != nil || len(items) < 2 || namespaces != nil {
		t.Fatalf("expected cluster list")
	}
	// namespaced: namespace role multiple namespaces error
	_, _, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default", "other"}}, matchNamespaced, "", "prov", "")
	if err == nil {
		t.Fatalf("expected multiple namespace error")
	}
	// namespaced: namespace role list
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"default"}}, matchNamespaced, "", "", "")
	if err != nil || len(items) == 0 || len(namespaces) != 1 {
		t.Fatalf("expected namespace role list")
	}
	// non-namespaced: name
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchCluster, "", "pool", "")
	if err != nil || len(items) != 1 || namespaces != nil {
		t.Fatalf("expected cluster get")
	}
	// non-namespaced: missing name
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchCluster, "", "missing", "")
	if err != nil || len(items) != 0 || namespaces != nil {
		t.Fatalf("expected cluster missing name")
	}
	// non-namespaced: list
	items, namespaces, err = toolset.listResourceObjects(context.Background(), policy.User{Role: policy.RoleCluster}, matchCluster, "", "", "")
	if err != nil || len(items) != 1 || namespaces != nil {
		t.Fatalf("expected cluster list")
	}
}
