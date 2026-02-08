package sdk

import (
	"context"
	"fmt"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"rootcause/internal/redact"
)

func TestResolveResourceWrapper(t *testing.T) {
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
	if gvr.Resource != "deployments" || !namespaced {
		t.Fatalf("unexpected gvr: %#v namespaced=%v", gvr, namespaced)
	}
}

type sdkDiscovery struct {
	resources []*metav1.APIResourceList
}

func (d *sdkDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return &metav1.APIGroupList{}, nil }
func (d *sdkDiscovery) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	for _, list := range d.resources {
		if list.GroupVersion == groupVersion {
			return list, nil
		}
	}
	return &metav1.APIResourceList{GroupVersion: groupVersion}, nil
}
func (d *sdkDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, d.resources, nil
}
func (d *sdkDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) { return d.resources, nil }
func (d *sdkDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return d.resources, nil
}
func (d *sdkDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *sdkDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *sdkDiscovery) OpenAPIV3() openapi.Client { return nil }
func (d *sdkDiscovery) RESTClient() rest.Interface { return nil }
func (d *sdkDiscovery) WithLegacy() discovery.DiscoveryInterface { return d }

var _ discovery.DiscoveryInterface = &sdkDiscovery{}

func TestResolveResourceBestEffortWrapper(t *testing.T) {
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
	discoveryClient := &sdkDiscovery{resources: []*metav1.APIResourceList{
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{{Name: "deployments", Kind: "Deployment", Namespaced: true}},
		},
	}}
	gvr, namespaced, err := ResolveResourceBestEffort(mapper, discoveryClient, "", "Deployment", "", "")
	if err != nil {
		t.Fatalf("resolve best effort: %v", err)
	}
	if gvr.Resource != "deployments" || !namespaced {
		t.Fatalf("unexpected gvr: %#v namespaced=%v", gvr, namespaced)
	}
}

func TestRegisterAndListToolsets(t *testing.T) {
	id := fmt.Sprintf("sdk-test-%d", time.Now().UnixNano())
	err := RegisterToolset(id, func() Toolset { return nil })
	if err != nil {
		t.Fatalf("register toolset: %v", err)
	}
	toolsets := RegisteredToolsets()
	found := false
	for _, name := range toolsets {
		if name == id {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected toolset id %s in list", id)
	}
}

func TestMustRegisterToolset(t *testing.T) {
	id := fmt.Sprintf("sdk-must-%d", time.Now().UnixNano())
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	MustRegisterToolset(id, func() Toolset { return nil })
}

type fakeCollector struct{}

func (fakeCollector) EventsForObject(context.Context, *unstructured.Unstructured) ([]corev1.Event, error) {
	return nil, nil
}

func (fakeCollector) OwnerChain(context.Context, *unstructured.Unstructured) ([]string, error) {
	return nil, nil
}

func (fakeCollector) PodStatusSummary(*corev1.Pod) map[string]any {
	return map[string]any{}
}

func (fakeCollector) RelatedPods(context.Context, string, labels.Selector) ([]corev1.Pod, error) {
	return nil, nil
}

func (fakeCollector) EndpointsForService(context.Context, string, string) (*corev1.Endpoints, error) {
	return nil, nil
}

func (fakeCollector) ResourceRef(gvr schema.GroupVersionResource, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", gvr.Resource, name)
	}
	return fmt.Sprintf("%s/%s/%s", gvr.Resource, namespace, name)
}

func TestDescribeAnalysisWrapper(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("demo")
	obj.SetNamespace("default")
	analysis := DescribeAnalysis(context.Background(), fakeCollector{}, redact.New(), schema.GroupVersionResource{Resource: "configmaps"}, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected evidence in analysis")
	}
}
