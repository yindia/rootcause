package linkerd

import (
	"context"
	"errors"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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

type emptyDiscovery struct{}

func (d *emptyDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return &metav1.APIGroupList{}, nil }
func (d *emptyDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return &metav1.APIResourceList{}, nil
}
func (d *emptyDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (d *emptyDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *emptyDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *emptyDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *emptyDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *emptyDiscovery) OpenAPIV3() openapi.Client { return nil }
func (d *emptyDiscovery) RESTClient() rest.Interface { return nil }
func (d *emptyDiscovery) Fresh() bool { return true }
func (d *emptyDiscovery) Invalidate() {}
func (d *emptyDiscovery) WithLegacy() discovery.DiscoveryInterface { return d }

var _ discovery.DiscoveryInterface = &emptyDiscovery{}

type errRegistry struct{}

func (errRegistry) Add(mcp.ToolSpec) error { return errors.New("add failed") }
func (errRegistry) List() []mcp.ToolInfo { return nil }
func (errRegistry) Get(string) (mcp.ToolSpec, bool) { return mcp.ToolSpec{}, false }

func newMinimalToolset(t *testing.T, discovery discovery.CachedDiscoveryInterface, typed *k8sfake.Clientset) *Toolset {
	t.Helper()
	cfg := config.DefaultConfig()
	clients := &kube.Clients{Typed: typed, Discovery: discovery}
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

func TestLinkerdHealthNotDetected(t *testing.T) {
	toolset := newMinimalToolset(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset())
	result, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence, got %#v", evidence)
	}
}

func TestLinkerdHealthNamespaceFallback(t *testing.T) {
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{},
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}},
	}
	toolset := newMinimalToolset(t, discoveryClient, k8sfake.NewSimpleClientset())
	result, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleHealth: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	found := false
	for _, item := range evidence {
		if item.Summary == "namespaceFallback" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected namespaceFallback evidence")
	}
}

func TestLinkerdProxyStatusNoProxies(t *testing.T) {
	toolset := newMinimalToolset(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
	))
	result, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
	})
	if err != nil {
		t.Fatalf("handleProxyStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence, got %#v", evidence)
	}
}

func TestLinkerdCRStatusNotDetected(t *testing.T) {
	toolset := newMinimalToolset(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset())
	result, err := toolset.handleCRStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"kind": "Foo"},
	})
	if err != nil {
		t.Fatalf("handleCRStatus: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence, got %#v", evidence)
	}
}

func TestLinkerdRegisterError(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{Clients: &kube.Clients{}}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := toolset.Register(errRegistry{}); err == nil {
		t.Fatalf("expected register error")
	}
}

func TestLinkerdToolsetFactory(t *testing.T) {
	factory, ok := mcp.ToolsetFactoryFor("linkerd")
	if !ok {
		t.Fatalf("expected toolset factory")
	}
	if factory() == nil {
		t.Fatalf("expected toolset instance")
	}
	if isGeneralMeshKind("", "") {
		t.Fatalf("expected empty kind/resource to be false")
	}
}
