package k8s

import (
	"context"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/version"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/discovery"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type vpaDiscovery struct{}

func (d *vpaDiscovery) ServerGroups() (*metav1.APIGroupList, error) { return &metav1.APIGroupList{}, nil }
func (d *vpaDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *vpaDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}
func (d *vpaDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *vpaDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}
func (d *vpaDiscovery) ServerVersion() (*version.Info, error) { return &version.Info{}, nil }
func (d *vpaDiscovery) OpenAPISchema() (*openapi_v2.Document, error) { return nil, nil }
func (d *vpaDiscovery) OpenAPIV3() openapi.Client                 { return nil }
func (d *vpaDiscovery) RESTClient() rest.Interface                { return nil }
func (d *vpaDiscovery) Fresh() bool                               { return true }
func (d *vpaDiscovery) Invalidate()                               {}
func (d *vpaDiscovery) WithLegacy() discovery.DiscoveryInterface  { return d }

func TestVPAHelpers(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"targetRef": map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "name": "api"},
		},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "RecommendationProvided", "status": "True"}},
			"recommendation": map[string]any{"containerRecommendations": []any{map[string]any{"containerName": "app", "target": map[string]any{"cpu": "100m"}}}},
		},
	}}
	ref := extractVPATargetRef(obj)
	if ref["name"] != "api" {
		t.Fatalf("unexpected target ref: %#v", ref)
	}
	if len(extractVPAConditions(obj)) == 0 {
		t.Fatalf("expected conditions")
	}
	if len(extractVPARecommendations(obj)) == 0 {
		t.Fatalf("expected recommendations")
	}
	if _, ok := nestedMap(obj, "spec", "targetRef"); !ok {
		t.Fatalf("expected nested map")
	}
	if len(stringifyResourceMap(map[string]any{"cpu": "100m"})) == 0 {
		t.Fatalf("expected resource map")
	}
}

func TestSelectorAndMetricsCollection(t *testing.T) {
	labels := map[string]string{"app": "api"}
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default", Labels: labels}}
	metric := &metricsv1beta1.PodMetrics{
		TypeMeta:   metav1.TypeMeta{Kind: "PodMetrics", APIVersion: "metrics.k8s.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Timestamp:  metav1.NewTime(time.Now()),
		Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("64Mi")}}},
	}

	client := k8sfake.NewSimpleClientset(deploy, pod)
	metricsClient := metricsfake.NewSimpleClientset(metric)
	clients := &kube.Clients{Typed: client, Metrics: metricsClient}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	selector, workload, err := toolset.selectorForTarget(context.Background(), "default", "Deployment", "api")
	if err != nil || selector == nil || workload == nil {
		t.Fatalf("selectorForTarget: %v", err)
	}

	evidence, err := toolset.collectVPATargetMetrics(context.Background(), "default", map[string]string{"kind": "Deployment", "name": "api"})
	if err != nil {
		t.Fatalf("collectVPATargetMetrics: %v", err)
	}
	if evidence == nil {
		t.Fatalf("expected metrics evidence")
	}
}

func TestHandleVPADebugNotDetected(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{Discovery: &vpaDiscovery{}}, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})
	_, err := toolset.handleVPADebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}, Arguments: map[string]any{"namespace": "default"}})
	if err != nil {
		t.Fatalf("handleVPADebug: %v", err)
	}
}
