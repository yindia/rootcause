package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestAddServiceGraphMissingEndpointsAndSelector(t *testing.T) {
	namespace := "default"
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace}}
	cache := newGraphCache()
	cache.servicesLoaded = true
	cache.services["api"] = service
	cache.serviceList = append(cache.serviceList, service)
	cache.endpointsLoaded = true

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: fake.NewSimpleClientset()}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()

	if _, err := toolset.addServiceGraph(context.Background(), graph, namespace, "api", cache); err != nil {
		t.Fatalf("addServiceGraph: %v", err)
	}
}

func TestAddDeploymentGraphSkipsUnownedReplicaSet(t *testing.T) {
	namespace := "default"
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "api-rs", Namespace: namespace},
		Spec:       appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}},
	}

	cache := newGraphCache()
	cache.deploymentsLoaded = true
	cache.deployments["api"] = deployment
	cache.deploymentList = append(cache.deploymentList, deployment)
	cache.replicasetsLoaded = true
	cache.replicasets["api-rs"] = rs
	cache.replicasetList = append(cache.replicasetList, rs)
	cache.servicesLoaded = true

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: fake.NewSimpleClientset()}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()

	if _, err := toolset.addDeploymentGraph(context.Background(), graph, namespace, "api", cache); err != nil {
		t.Fatalf("addDeploymentGraph: %v", err)
	}
}

func TestAddReplicaSetPodsNilMore(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: fake.NewSimpleClientset()}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()
	if _, err := toolset.addReplicaSetPods(context.Background(), graph, "default", nil, newGraphCache()); err != nil {
		t.Fatalf("addReplicaSetPods nil: %v", err)
	}
}

func TestAddStatefulSetGraphHeadlessServiceMissing(t *testing.T) {
	namespace := "default"
	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}},
			ServiceName: "headless",
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "db-0", Namespace: namespace, Labels: map[string]string{"app": "db"}}}

	cache := newGraphCache()
	cache.statefulsetsLoaded = true
	cache.statefulsets["db"] = ss
	cache.statefulsetList = append(cache.statefulsetList, ss)
	cache.podsLoaded = true
	cache.pods["db-0"] = pod
	cache.podList = append(cache.podList, pod)
	cache.servicesLoaded = true

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: fake.NewSimpleClientset()}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()

	if _, err := toolset.addStatefulSetGraph(context.Background(), graph, namespace, "db", cache); err != nil {
		t.Fatalf("addStatefulSetGraph: %v", err)
	}
}

func TestAddIngressGraphNoBackends(t *testing.T) {
	namespace := "default"
	ingress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: namespace}}
	cache := newGraphCache()
	cache.ingressesLoaded = true
	cache.ingresses["empty"] = ingress
	cache.ingressList = append(cache.ingressList, ingress)

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Typed: fake.NewSimpleClientset()}, Policy: policy.NewAuthorizer()})
	graph := newGraphBuilder()

	if _, err := toolset.addIngressGraph(context.Background(), graph, namespace, "empty", cache); err != nil {
		t.Fatalf("addIngressGraph: %v", err)
	}
}
