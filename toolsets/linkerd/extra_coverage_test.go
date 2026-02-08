package linkerd

import (
	"context"
	"fmt"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type errorDiscovery struct{}

func (d *errorDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return nil, fmt.Errorf("boom")
}
func (d *errorDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("boom")
}
func (d *errorDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, fmt.Errorf("boom")
}
func (d *errorDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("boom")
}
func (d *errorDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, fmt.Errorf("boom")
}
func (d *errorDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *errorDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *errorDiscovery) OpenAPIV3() openapi.Client { return nil }
func (d *errorDiscovery) RESTClient() rest.Interface { return nil }
func (d *errorDiscovery) Fresh() bool { return true }
func (d *errorDiscovery) Invalidate() {}
func (d *errorDiscovery) WithLegacy() discovery.DiscoveryInterface { return d }

var _ discovery.CachedDiscoveryInterface = &errorDiscovery{}

func newToolsetWithDiscovery(t *testing.T, discovery discovery.CachedDiscoveryInterface, typed *k8sfake.Clientset) *Toolset {
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

func TestLinkerdHealthDetectError(t *testing.T) {
	toolset := newToolsetWithDiscovery(t, &errorDiscovery{}, k8sfake.NewSimpleClientset())
	if _, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected detect error")
	}
}

func TestLinkerdPolicyDebugDetectError(t *testing.T) {
	toolset := newToolsetWithDiscovery(t, &errorDiscovery{}, k8sfake.NewSimpleClientset())
	if _, err := toolset.handlePolicyDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected detect error")
	}
}

func TestLinkerdPolicyDebugNotDetected(t *testing.T) {
	toolset := newToolsetWithDiscovery(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset())
	result, err := toolset.handlePolicyDebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handlePolicyDebug: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence")
	}
}

func TestLinkerdProxyStatusNamespaceDenied(t *testing.T) {
	toolset := newToolsetWithDiscovery(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}))
	_, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleNamespace, AllowedNamespaces: []string{"other"}},
		Arguments: map[string]any{"namespace": "default"},
	})
	if err == nil {
		t.Fatalf("expected namespace error")
	}
}

func TestLinkerdHealthDeploymentListError(t *testing.T) {
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("list", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("list fail")
	})
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{},
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}},
	}
	toolset := newToolsetWithDiscovery(t, discoveryClient, client)
	if _, err := toolset.handleHealth(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected list error")
	}
}

func TestLinkerdProxyStatusListError(t *testing.T) {
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	client.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("list fail")
	})
	toolset := newToolsetWithDiscovery(t, &emptyDiscovery{}, client)
	if _, err := toolset.handleProxyStatus(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"namespace": "default"},
	}); err == nil {
		t.Fatalf("expected pods list error")
	}
}

func TestLinkerdIdentityIssuesNotDetected(t *testing.T) {
	toolset := newToolsetWithDiscovery(t, &emptyDiscovery{}, k8sfake.NewSimpleClientset())
	result, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleIdentityIssues: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence")
	}
}

func TestLinkerdIdentityIssuesDeploymentMissing(t *testing.T) {
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{},
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}},
	}
	toolset := newToolsetWithDiscovery(t, discoveryClient, k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}}))
	result, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleIdentityIssues: %v", err)
	}
	data := result.Data.(map[string]any)
	evidence := data["evidence"].([]render.EvidenceItem)
	if len(evidence) == 0 || evidence[0].Summary != "status" {
		t.Fatalf("expected status evidence for missing deployment")
	}
}

func TestDetectLinkerdNamespaceExists(t *testing.T) {
	discoveryClient := &emptyDiscovery{}
	client := k8sfake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}})
	toolset := newToolsetWithDiscovery(t, discoveryClient, client)
	detected, namespaces, _, err := toolset.detectLinkerd(context.Background())
	if err != nil {
		t.Fatalf("detectLinkerd: %v", err)
	}
	if !detected || len(namespaces) != 1 || namespaces[0] != "linkerd" {
		t.Fatalf("unexpected detect result: %v %#v", detected, namespaces)
	}
}

func TestLinkerdIdentityIssuesNotReady(t *testing.T) {
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "linkerd-identity",
			Namespace: "linkerd",
		},
		Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 0},
	}
	discoveryClient := &linkerdDiscoveryResources{
		resources: []*metav1.APIResourceList{},
		groups:    &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "linkerd.io"}}},
	}
	client := k8sfake.NewSimpleClientset(
		deploy,
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "linkerd"}},
	)
	toolset := newToolsetWithDiscovery(t, discoveryClient, client)
	result, err := toolset.handleIdentityIssues(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err != nil {
		t.Fatalf("handleIdentityIssues: %v", err)
	}
	data := result.Data.(map[string]any)
	causes := data["likelyRootCauses"].([]render.Cause)
	if len(causes) == 0 {
		t.Fatalf("expected root causes for not ready identity")
	}
}
