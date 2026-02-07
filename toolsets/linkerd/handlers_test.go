package linkerd

import (
	"context"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

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

type linkerdDiscovery struct {
	groups []string
}

func (d *linkerdDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	list := &metav1.APIGroupList{}
	for _, name := range d.groups {
		list.Groups = append(list.Groups, metav1.APIGroup{Name: name})
	}
	return list, nil
}

func (d *linkerdDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *linkerdDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (d *linkerdDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *linkerdDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *linkerdDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (d *linkerdDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *linkerdDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *linkerdDiscovery) RESTClient() rest.Interface {
	return nil
}

func (d *linkerdDiscovery) Fresh() bool {
	return true
}

func (d *linkerdDiscovery) Invalidate() {}

func (d *linkerdDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &linkerdDiscovery{}

func TestHandleProxyStatus(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "linkerd-proxy"},
			},
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodReady, Status: corev1.ConditionTrue},
			},
		},
	}
	client := fake.NewSimpleClientset(pod)
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

	result, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		Arguments: map[string]any{
			"namespace": "default",
		},
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence in output")
	}
}

func TestHandleHealthNotDetected(t *testing.T) {
	client := fake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client, Discovery: &linkerdDiscovery{groups: []string{}}}
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

	result, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["evidence"] == nil {
		t.Fatalf("expected evidence output, got %#v", result.Data)
	}
}
