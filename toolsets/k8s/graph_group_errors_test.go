package k8s

import (
	"context"
	"errors"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type errorDiscoveryGroup struct{}

func (d *errorDiscoveryGroup) ServerGroups() (*metav1.APIGroupList, error)            { return &metav1.APIGroupList{}, nil }
func (d *errorDiscoveryGroup) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, errors.New("discovery failed")
}
func (d *errorDiscoveryGroup) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, errors.New("discovery failed")
}
func (d *errorDiscoveryGroup) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, errors.New("discovery failed")
}
func (d *errorDiscoveryGroup) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, errors.New("discovery failed")
}
func (d *errorDiscoveryGroup) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *errorDiscoveryGroup) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *errorDiscoveryGroup) OpenAPIV3() openapi.Client              { return nil }
func (d *errorDiscoveryGroup) RESTClient() rest.Interface             { return nil }
func (d *errorDiscoveryGroup) Fresh() bool                            { return true }
func (d *errorDiscoveryGroup) Invalidate()                            {}
func (d *errorDiscoveryGroup) WithLegacy() discovery.DiscoveryInterface { return d }

func TestAddGroupResourcesDiscoveryError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: &errorDiscoveryGroup{}},
		Policy:  policy.NewAuthorizer(),
	})
	graph := newGraphBuilder()
	warnings := toolset.addGroupResources(context.Background(), graph, "default", "example.com", map[string]string{}, nil)
	if len(warnings) == 0 {
		t.Fatalf("expected discovery warning")
	}
}

func TestAddMeshGraphResolveErrors(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:  &cfg,
		Clients: &kube.Clients{Discovery: &errorDiscoveryGroup{}},
		Policy:  policy.NewAuthorizer(),
	})
	graph := newGraphBuilder()
	if warnings := toolset.addGatewayAPIGraph(context.Background(), graph, "default", map[string]string{}); len(warnings) == 0 {
		t.Fatalf("expected gateway api resolve warning")
	}
	if warnings := toolset.addIstioGraph(context.Background(), graph, "default", map[string]string{}); len(warnings) == 0 {
		t.Fatalf("expected istio resolve warning")
	}
}

var _ discovery.CachedDiscoveryInterface = &errorDiscoveryGroup{}
