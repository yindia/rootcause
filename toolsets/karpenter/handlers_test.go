package karpenter

import (
	"context"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/fake"
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

type karpenterDiscovery struct {
	groups []string
}

func (d *karpenterDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	list := &metav1.APIGroupList{}
	for _, name := range d.groups {
		list.Groups = append(list.Groups, metav1.APIGroup{Name: name})
	}
	return list, nil
}

func (d *karpenterDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *karpenterDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (d *karpenterDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *karpenterDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *karpenterDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (d *karpenterDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *karpenterDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *karpenterDiscovery) RESTClient() rest.Interface {
	return nil
}

func (d *karpenterDiscovery) Fresh() bool {
	return true
}

func (d *karpenterDiscovery) Invalidate() {}

func (d *karpenterDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &karpenterDiscovery{}

func TestHandleStatusDetected(t *testing.T) {
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karpenter",
			Namespace: "karpenter",
			Labels:    map[string]string{"app.kubernetes.io/name": "karpenter"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "karpenter"}}
	client := fake.NewSimpleClientset(deploy, ns)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &karpenterDiscovery{groups: []string{"karpenter.sh"}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})

	result, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence in output")
	}
	if len(result.Metadata.Namespaces) != 1 || result.Metadata.Namespaces[0] != "karpenter" {
		t.Fatalf("unexpected namespaces: %#v", result.Metadata.Namespaces)
	}
}

func TestHandleStatusNotDetected(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &karpenterDiscovery{groups: []string{}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})

	result, err := toolset.handleStatus(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleStatus: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["evidence"] == nil {
		t.Fatalf("expected evidence output, got %#v", result.Data)
	}
}
