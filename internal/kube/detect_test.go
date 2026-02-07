package kube

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
)

type groupsDiscovery struct {
	groups []string
}

func (g *groupsDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	list := &metav1.APIGroupList{}
	for _, name := range g.groups {
		list.Groups = append(list.Groups, metav1.APIGroup{Name: name})
	}
	return list, nil
}

func (g *groupsDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}
func (g *groupsDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (g *groupsDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (g *groupsDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (g *groupsDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}
func (g *groupsDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}
func (g *groupsDiscovery) OpenAPIV3() openapi.Client {
	return nil
}
func (g *groupsDiscovery) RESTClient() rest.Interface {
	return nil
}
func (g *groupsDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return g
}

var _ discovery.DiscoveryInterface = &groupsDiscovery{}

func TestGroupsPresent(t *testing.T) {
	found, groups, err := GroupsPresent(&groupsDiscovery{groups: []string{"apps", "karpenter.sh"}}, []string{"karpenter.sh"})
	if err != nil {
		t.Fatalf("groups present: %v", err)
	}
	if !found || len(groups) != 1 || groups[0] != "karpenter.sh" {
		t.Fatalf("unexpected groups: %#v", groups)
	}
}

func TestGroupsPresentMissingClient(t *testing.T) {
	_, _, err := GroupsPresent(nil, []string{"apps"})
	if err == nil {
		t.Fatalf("expected error for missing discovery client")
	}
}

func TestControlPlaneNamespaces(t *testing.T) {
	ctx := context.Background()
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "mesh",
			Labels:    map[string]string{"app": "demo"},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-pod",
			Namespace: "mesh2",
			Labels:    map[string]string{"app": "demo"},
		},
	}
	client := fake.NewSimpleClientset(deploy, pod)
	clients := &Clients{Typed: client}

	namespaces, err := ControlPlaneNamespaces(ctx, clients, []string{"app=demo"})
	if err != nil {
		t.Fatalf("control plane namespaces: %v", err)
	}
	if len(namespaces) != 1 || namespaces[0] != "mesh" {
		t.Fatalf("expected deployment namespace, got %#v", namespaces)
	}

	namespaces, err = ControlPlaneNamespaces(ctx, clients, []string{"app=missing"})
	if err != nil {
		t.Fatalf("control plane namespaces: %v", err)
	}
	if len(namespaces) != 0 {
		t.Fatalf("expected no namespaces, got %#v", namespaces)
	}

	namespaces, err = ControlPlaneNamespaces(ctx, clients, []string{"app=demo", "app=missing"})
	if err != nil {
		t.Fatalf("control plane namespaces: %v", err)
	}
	if len(namespaces) == 0 {
		t.Fatalf("expected namespaces from deployments or pods")
	}
}

func TestControlPlaneNamespacesPodFallback(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-pod",
			Namespace: "mesh",
			Labels:    map[string]string{"app": "demo"},
		},
	}
	client := fake.NewSimpleClientset(pod)
	clients := &Clients{Typed: client}

	namespaces, err := ControlPlaneNamespaces(ctx, clients, []string{"app=demo"})
	if err != nil {
		t.Fatalf("control plane namespaces: %v", err)
	}
	if len(namespaces) != 1 || namespaces[0] != "mesh" {
		t.Fatalf("expected pod namespace, got %#v", namespaces)
	}
}

func TestControlPlaneNamespacesMissingClient(t *testing.T) {
	_, err := ControlPlaneNamespaces(context.Background(), &Clients{}, []string{"app=demo"})
	if err == nil {
		t.Fatalf("expected error for missing typed client")
	}
}
