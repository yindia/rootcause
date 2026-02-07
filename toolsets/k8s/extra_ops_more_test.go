package k8s

import (
	"context"
	"errors"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type errorVersionDiscovery struct{}

func (d *errorVersionDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return &metav1.APIGroupList{}, nil }
func (d *errorVersionDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return &metav1.APIResourceList{}, nil
}
func (d *errorVersionDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (d *errorVersionDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *errorVersionDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *errorVersionDiscovery) ServerVersion() (*version.Info, error) { return nil, errors.New("boom") }
func (d *errorVersionDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *errorVersionDiscovery) OpenAPIV3() openapi.Client                 { return nil }
func (d *errorVersionDiscovery) RESTClient() rest.Interface                 { return nil }
func (d *errorVersionDiscovery) Fresh() bool                                { return true }
func (d *errorVersionDiscovery) Invalidate()                                {}
func (d *errorVersionDiscovery) WithLegacy() discovery.DiscoveryInterface   { return d }

func TestHandleCreateAndScaleErrors(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	toolset := newDynamicToolset(gvr, "ConfigMapList", "ConfigMap")

	if _, err := toolset.handleCreate(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"confirm": true},
	}); err == nil {
		t.Fatalf("expected handleCreate missing manifest error")
	}

	if _, err := toolset.handleCreate(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"confirm":  true,
			"manifest": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n  namespace: default\n",
			"namespace": "other",
		},
	}); err == nil {
		t.Fatalf("expected handleCreate namespace mismatch error")
	}

	if _, err := toolset.handleScale(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"confirm": true},
	}); err == nil {
		t.Fatalf("expected handleScale missing args error")
	}
}

func TestHandleRolloutUnsupportedAction(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handleRollout(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"action":    "pause",
			"name":      "api",
			"namespace": "default",
			"confirm":   true,
		},
	}); err == nil {
		t.Fatalf("expected handleRollout unsupported action error")
	}
}

func TestHandlePingError(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Discovery: &errorVersionDiscovery{}}, Policy: policy.NewAuthorizer()})
	if _, err := toolset.handlePing(context.Background(), mcp.ToolRequest{}); err == nil {
		t.Fatalf("expected handlePing error")
	}
}
