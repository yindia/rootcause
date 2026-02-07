package k8s

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/discovery/cached/memory"
	discoveryfake "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
	clienttesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestHandleVPADebugDetected(t *testing.T) {
	namespace := "default"
	labels := map[string]string{"app": "api"}
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}},
		},
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: labels}}
	metric := &metricsv1beta1.PodMetrics{
		TypeMeta:   metav1.TypeMeta{Kind: "PodMetrics", APIVersion: "metrics.k8s.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace},
		Timestamp:  metav1.NewTime(time.Now()),
		Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("64Mi")}}},
	}

	vpa := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "autoscaling.k8s.io/v1",
		"kind":       "VerticalPodAutoscaler",
		"metadata":   map[string]any{"name": "api-vpa", "namespace": namespace},
		"spec": map[string]any{
			"targetRef": map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "name": "api"},
		},
		"status": map[string]any{
			"recommendation": map[string]any{"containerRecommendations": []any{map[string]any{"containerName": "app", "target": map[string]any{"cpu": "100m"}}}},
		},
	}}

	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{gvr: "VerticalPodAutoscalerList"}, vpa)

	resources := []*metav1.APIResourceList{{
		GroupVersion: "autoscaling.k8s.io/v1",
		APIResources: []metav1.APIResource{{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
	}}
	discovery := &discoveryfake.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}}
	cached := memory.NewMemCacheClient(discovery)
	mapper := restmapper.NewDiscoveryRESTMapper([]*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{Name: "autoscaling.k8s.io", Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "autoscaling.k8s.io/v1", Version: "v1"}},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {{Name: "verticalpodautoscalers", Kind: "VerticalPodAutoscaler", Namespaced: true}},
			},
		},
	})

	client := k8sfake.NewSimpleClientset(deploy, pod)
	metricsClient := metricsfake.NewSimpleClientset(metric)
	clients := &kube.Clients{Typed: client, Dynamic: dyn, Discovery: cached, Mapper: mapper, Metrics: metricsClient}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: clients, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New(), Evidence: evidence.NewCollector(clients)})

	_, err := toolset.handleVPADebug(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}, Arguments: map[string]any{"namespace": namespace}})
	if err != nil {
		t.Fatalf("handleVPADebug detected: %v", err)
	}
}
