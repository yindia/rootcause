package kube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/homedir"
)

type fakeDiscovery struct {
	resources   []*metav1.APIResourceList
	invalidated bool
}

func (f *fakeDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{}, nil
}

func (f *fakeDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, res := range f.resources {
		if res.GroupVersion == groupVersion {
			return res, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}

func (f *fakeDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, f.resources, nil
}

func (f *fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.resources, nil
}

func (f *fakeDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return f.resources, nil
}

func (f *fakeDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (f *fakeDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (f *fakeDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (f *fakeDiscovery) RESTClient() rest.Interface {
	return nil
}

func (f *fakeDiscovery) Fresh() bool {
	return true
}

func (f *fakeDiscovery) Invalidate() {
	f.invalidated = true
}

func (f *fakeDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return f
}

var _ discovery.CachedDiscoveryInterface = &fakeDiscovery{}

func TestKubeconfigPathTilde(t *testing.T) {
	home := homedir.HomeDir()
	if home == "" {
		t.Skip("home dir not available")
	}
	path := kubeconfigPath("~/.kube/config")
	if !strings.HasPrefix(path, home) {
		t.Fatalf("expected home-expanded path, got %q", path)
	}
}

func TestKubeconfigPathEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KUBEPATH", dir)
	path := kubeconfigPath("$KUBEPATH/config")
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("expected env-expanded path, got %q", path)
	}
}

func TestResolveResource(t *testing.T) {
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "deployments", Kind: "Deployment", Namespaced: true}},
			},
		},
	})

	gvr, namespaced, err := ResolveResource(mapper, "apps/v1", "Deployment", "")
	if err != nil {
		t.Fatalf("resolve resource: %v", err)
	}
	if gvr.Resource != "deployments" {
		t.Fatalf("expected deployments, got %s", gvr.Resource)
	}
	if !namespaced {
		t.Fatalf("expected namespaced true")
	}
}

func TestResolveResourceWithResource(t *testing.T) {
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{GroupVersion: "apps/v1", Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "apps/v1", Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "deployments", Kind: "Deployment", Namespaced: true}},
			},
		},
	})

	gvr, namespaced, err := ResolveResource(mapper, "apps/v1", "", "deployments")
	if err != nil {
		t.Fatalf("resolve resource: %v", err)
	}
	if gvr.Resource != "deployments" {
		t.Fatalf("expected deployments, got %s", gvr.Resource)
	}
	if !namespaced {
		t.Fatalf("expected namespaced true")
	}
}

func TestResolveResourceInvalidAPIVersion(t *testing.T) {
	mapper := restmapper.NewDiscoveryRESTMapper(nil)
	_, _, err := ResolveResource(mapper, "invalid", "Pod", "")
	if err == nil {
		t.Fatalf("expected error for invalid apiVersion")
	}
}

func TestResolveResourceErrors(t *testing.T) {
	_, _, err := ResolveResource(nil, "v1", "Pod", "")
	if err == nil {
		t.Fatalf("expected error for nil mapper")
	}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)
	_, _, err = ResolveResource(mapper, "", "", "")
	if err == nil {
		t.Fatalf("expected error for missing apiVersion/kind")
	}
}

func TestResolveResourceBestEffort(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
	}
	fake := &fakeDiscovery{resources: resources}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)

	gvr, namespaced, err := ResolveResourceBestEffort(mapper, fake, "", "Deployment", "", "")
	if err != nil {
		t.Fatalf("resolve best effort: %v", err)
	}
	if gvr.Resource != "deployments" {
		t.Fatalf("expected deployments, got %s", gvr.Resource)
	}
	if !namespaced {
		t.Fatalf("expected namespaced true")
	}
}

func TestResolveResourceBestEffortShortName(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true, ShortNames: []string{"deploy"}},
			},
		},
	}
	fake := &fakeDiscovery{resources: resources}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)

	gvr, namespaced, err := ResolveResourceBestEffort(mapper, fake, "", "", "deploy", "")
	if err != nil {
		t.Fatalf("resolve best effort: %v", err)
	}
	if gvr.Resource != "deployments" {
		t.Fatalf("expected deployments, got %s", gvr.Resource)
	}
	if !namespaced {
		t.Fatalf("expected namespaced true")
	}
}

func TestResolveResourceBestEffortQualifiedResource(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
	}
	fake := &fakeDiscovery{resources: resources}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)

	gvr, namespaced, err := ResolveResourceBestEffort(mapper, fake, "", "", "deployments.apps", "")
	if err != nil {
		t.Fatalf("resolve best effort: %v", err)
	}
	if gvr.Group != "apps" || gvr.Resource != "deployments" {
		t.Fatalf("unexpected gvr: %#v", gvr)
	}
	if !namespaced {
		t.Fatalf("expected namespaced true")
	}
}

func TestResolveResourceBestEffortMultipleMatches(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
		{
			GroupVersion: "extensions/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
	}
	fake := &fakeDiscovery{resources: resources}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)

	_, _, err := ResolveResourceBestEffort(mapper, fake, "", "", "deployments", "")
	if err == nil {
		t.Fatalf("expected error for multiple matches")
	}
}

func TestResolveResourceBestEffortNoMatches(t *testing.T) {
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
			},
		},
	}
	fake := &fakeDiscovery{resources: resources}
	mapper := restmapper.NewDiscoveryRESTMapper(nil)

	_, _, err := ResolveResourceBestEffort(mapper, fake, "", "Missing", "", "")
	if err == nil {
		t.Fatalf("expected error for missing kind")
	}
}

func TestResolveResourceBestEffortErrors(t *testing.T) {
	mapper := restmapper.NewDiscoveryRESTMapper(nil)
	_, _, err := ResolveResourceBestEffort(nil, nil, "", "Pod", "", "")
	if err == nil {
		t.Fatalf("expected error for missing mapper")
	}
	_, _, err = ResolveResourceBestEffort(mapper, nil, "", "Pod", "", "")
	if err == nil {
		t.Fatalf("expected error for missing discovery client")
	}
	_, _, err = ResolveResourceBestEffort(mapper, &fakeDiscovery{}, "", "", "", "")
	if err == nil {
		t.Fatalf("expected error for missing kind/resource")
	}
}

func TestRefreshDiscovery(t *testing.T) {
	fake := &fakeDiscovery{}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(fake)
	clients := &Clients{
		Discovery:          fake,
		deferredRESTMapper: mapper,
		discoveryResetAt:   time.Now().Add(-2 * time.Second),
	}
	clients.RefreshDiscovery(time.Second)
	if !fake.invalidated {
		t.Fatalf("expected discovery invalidate to be called")
	}
}

func TestRefreshDiscoveryNoop(t *testing.T) {
	fake := &fakeDiscovery{}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(fake)
	clients := &Clients{
		Discovery:          fake,
		deferredRESTMapper: mapper,
		discoveryResetAt:   time.Now(),
	}
	clients.RefreshDiscovery(10 * time.Second)
	if fake.invalidated {
		t.Fatalf("expected no invalidation when ttl not exceeded")
	}
}

func TestNewClientsFromKubeconfig(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	clients, err := NewClients(Config{Kubeconfig: kubeconfigPath})
	if err != nil {
		t.Fatalf("new clients: %v", err)
	}
	if clients == nil || clients.Typed == nil || clients.Dynamic == nil || clients.Discovery == nil || clients.Mapper == nil {
		t.Fatalf("expected clients to be initialized")
	}
}

func TestNewClients(t *testing.T) {
	dir := t.TempDir()
	kubeconfigPath := filepath.Join(dir, "kubeconfig")
	kubeconfig := `
apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: https://example.com
users:
- name: test
  user:
    token: fake
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`
	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	clients, err := NewClients(Config{Kubeconfig: kubeconfigPath, Context: "test"})
	if err != nil {
		t.Fatalf("new clients: %v", err)
	}
	if clients.Typed == nil || clients.Dynamic == nil || clients.Mapper == nil {
		t.Fatalf("expected clients to be initialized")
	}
}
