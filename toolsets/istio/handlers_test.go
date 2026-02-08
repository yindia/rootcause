package istio

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

type istioDiscovery struct {
	groups []string
}

func (d *istioDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	list := &metav1.APIGroupList{}
	for _, name := range d.groups {
		list.Groups = append(list.Groups, metav1.APIGroup{Name: name})
	}
	return list, nil
}

func (d *istioDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *istioDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (d *istioDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *istioDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *istioDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (d *istioDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *istioDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *istioDiscovery) RESTClient() rest.Interface {
	return nil
}

func (d *istioDiscovery) Fresh() bool {
	return true
}

func (d *istioDiscovery) Invalidate() {}

func (d *istioDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

var _ discovery.CachedDiscoveryInterface = &istioDiscovery{}

func TestHandleHealthDetected(t *testing.T) {
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istiod",
			Namespace: "istio-system",
			Labels:    map[string]string{"app": "istiod"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "istio-system"}}
	client := fake.NewSimpleClientset(deploy, ns)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &istioDiscovery{groups: []string{"networking.istio.io"}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})

	result, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result.Data)
	}
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence in output")
	}
	if len(result.Metadata.Namespaces) != 1 || result.Metadata.Namespaces[0] != "istio-system" {
		t.Fatalf("unexpected namespaces: %#v", result.Metadata.Namespaces)
	}
}

func TestHandleHealthNotDetected(t *testing.T) {
	client := fake.NewSimpleClientset()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Typed: client, Discovery: &istioDiscovery{groups: []string{}}},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Evidence: evidence.NewCollector(&kube.Clients{Typed: client}),
	})
	result, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	data := result.Data.(map[string]any)
	if evidenceItems, ok := data["evidence"].([]render.EvidenceItem); ok && len(evidenceItems) == 0 {
		t.Fatalf("expected evidence")
	}
}

func TestHandleProxyStatusNoProxy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
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
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	if _, ok := data["evidence"]; !ok {
		t.Fatalf("expected evidence output")
	}
}
