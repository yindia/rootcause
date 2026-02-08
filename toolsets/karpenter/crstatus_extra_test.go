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
	"k8s.io/client-go/restmapper"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func newCRStatusToolset(t *testing.T, objects ...runtime.Object) *Toolset {
	t.Helper()
	gvrProv := schema.GroupVersionResource{Group: "karpenter.sh", Version: "v1beta1", Resource: "provisioners"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvrProv: "ProvisionerList",
	}, objects...)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "karpenter.sh",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "karpenter.sh/v1beta1", Version: "v1beta1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "karpenter.sh/v1beta1", Version: "v1beta1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1beta1": {{Name: "provisioners", Kind: "Provisioner", Namespaced: true}},
			},
		},
	})
	discoveryClient := &fakeCachedDiscovery{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "karpenter.sh/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "provisioners", Kind: "Provisioner", Namespaced: true},
				},
			},
		},
		groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "karpenter.sh"}}},
	}
	typed := k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
	)
	clients := &kube.Clients{Typed: typed, Dynamic: dynamicClient, Discovery: discoveryClient, Mapper: mapper}
	cfg := config.DefaultConfig()
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Evidence: evidence.NewCollector(clients),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return toolset
}

func TestKarpenterCRStatusNotDetected(t *testing.T) {
	toolset := newCRStatusToolset(t)
	toolset.ctx.Clients.Discovery = &fakeCachedDiscovery{groups: &metav1.APIGroupList{}}
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Provisioner"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence")
	}
}

func TestKarpenterCRStatusNotFoundName(t *testing.T) {
	toolset := newCRStatusToolset(t)
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Provisioner", "name": "missing", "namespace": "default"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected status evidence for missing")
	}
}

func TestKarpenterCRStatusSingleFound(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("karpenter.sh/v1beta1")
	obj.SetKind("Provisioner")
	obj.SetName("prov")
	obj.SetNamespace("default")
	obj.Object["status"] = map[string]any{"phase": "Ready"}
	toolset := newCRStatusToolset(t, obj)
	if _, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Provisioner", "name": "prov"},
	}); err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
}
