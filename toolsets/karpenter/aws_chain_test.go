package karpenter

import (
	"context"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type fakeCachedDiscovery struct {
	resources []*metav1.APIResourceList
	groups    *metav1.APIGroupList
}

func (f *fakeCachedDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return f.groups, nil
}

func (f *fakeCachedDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, res := range f.resources {
		if res.GroupVersion == groupVersion {
			return res, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (f *fakeCachedDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	if f.groups == nil {
		return nil, f.resources, nil
	}
	out := make([]*metav1.APIGroup, 0, len(f.groups.Groups))
	for i := range f.groups.Groups {
		group := f.groups.Groups[i]
		out = append(out, &group)
	}
	return out, f.resources, nil
}

func (f *fakeCachedDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.resources, nil
}

func (f *fakeCachedDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return f.resources, nil
}

func (f *fakeCachedDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (f *fakeCachedDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (f *fakeCachedDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (f *fakeCachedDiscovery) RESTClient() rest.Interface {
	return nil
}

func (f *fakeCachedDiscovery) Fresh() bool {
	return true
}

func (f *fakeCachedDiscovery) Invalidate() {}

func (f *fakeCachedDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return f
}

var _ discovery.CachedDiscoveryInterface = &fakeCachedDiscovery{}

func TestNodeClassDebugCallsAWS(t *testing.T) {
	ctx := context.Background()

	nodeClass := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "karpenter.k8s.aws/v1beta1",
			"kind":       "EC2NodeClass",
			"metadata": map[string]any{
				"name": "default",
			},
			"spec": map[string]any{
				"role":            "arn:aws:iam::123456789012:role/KarpenterNodeRole",
				"instanceProfile": "arn:aws:iam::123456789012:instance-profile/KarpenterProfile",
				"subnetSelectorTerms": []any{
					map[string]any{
						"ids": []any{"subnet-1234"},
						"tags": map[string]any{
							"karpenter.sh/discovery": "cluster",
						},
					},
				},
				"securityGroupSelectorTerms": []any{
					map[string]any{
						"tags": map[string]any{
							"karpenter.sh/discovery": "cluster",
						},
					},
				},
			},
		},
	}

	gvr := schema.GroupVersionResource{Group: "karpenter.k8s.aws", Version: "v1beta1", Resource: "ec2nodeclasses"}
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "EC2NodeClassList",
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
		groups: &metav1.APIGroupList{
			Groups: []metav1.APIGroup{
				{
					Name: "karpenter.sh",
					Versions: []metav1.GroupVersionForDiscovery{
						{GroupVersion: "karpenter.sh/v1beta1", Version: "v1beta1"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "karpenter.sh/v1beta1", Version: "v1beta1"},
				},
			},
		},
	}

	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	calls := map[string]int{}
	addStub := func(name string) {
		_ = reg.Add(mcp.ToolSpec{
			Name:      name,
			ToolsetID: "aws",
			Safety:    mcp.SafetyReadOnly,
			Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
				calls[name]++
				return mcp.ToolResult{Data: map[string]any{"ok": true}}, nil
			},
		})
	}
	addStub("aws.vpc.list_subnets")
	addStub("aws.vpc.list_security_groups")
	addStub("aws.iam.get_role")
	addStub("aws.iam.get_instance_profile")

	clients := &kube.Clients{
		Dynamic:   dynamicClient,
		Discovery: discoveryClient,
		Typed:     kubefake.NewSimpleClientset(),
	}
	toolCtx := mcp.ToolContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Evidence: evidence.NewCollector(clients),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Registry: reg,
	}
	toolCtx.Invoker = mcp.NewToolInvoker(reg, toolCtx)

	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext(toolCtx)); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	_, err := toolset.handleNodeClassDebug(ctx, mcp.ToolRequest{
		Arguments: map[string]any{},
		User:      policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleNodeClassDebug failed: %v", err)
	}

	if calls["aws.vpc.list_subnets"] == 0 {
		t.Fatalf("expected aws.vpc.list_subnets to be called")
	}
	if calls["aws.vpc.list_security_groups"] == 0 {
		t.Fatalf("expected aws.vpc.list_security_groups to be called")
	}
	if calls["aws.iam.get_role"] == 0 {
		t.Fatalf("expected aws.iam.get_role to be called")
	}
	if calls["aws.iam.get_instance_profile"] == 0 {
		t.Fatalf("expected aws.iam.get_instance_profile to be called")
	}
}
