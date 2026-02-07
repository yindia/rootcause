package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

func TestCollectVPATargetMetricsMissingMetricsClient(t *testing.T) {
	namespace := "default"
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: map[string]string{"app": "api"}},
	}
	client := fake.NewSimpleClientset(deploy, pod)
	clients := &kube.Clients{Typed: client}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	evidence, err := toolset.collectVPATargetMetrics(context.Background(), namespace, map[string]string{"kind": "Deployment", "name": "api"})
	if err != nil {
		t.Fatalf("collectVPATargetMetrics: %v", err)
	}
	if evidence == nil || evidence["metrics"] == nil {
		t.Fatalf("expected missing metrics evidence")
	}
}

func TestCollectVPATargetMetricsNotFound(t *testing.T) {
	namespace := "default"
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: map[string]string{"app": "api"}},
	}
	client := fake.NewSimpleClientset(deploy, pod)
	metricsClient := metricsfake.NewSimpleClientset()
	metricsClient.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "pods"}, "")
	})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	evidence, err := toolset.collectVPATargetMetrics(context.Background(), namespace, map[string]string{"kind": "Deployment", "name": "api"})
	if err != nil {
		t.Fatalf("collectVPATargetMetrics not found: %v", err)
	}
	if evidence == nil || evidence["metrics"] == nil {
		t.Fatalf("expected metrics not found evidence")
	}
}

func TestCollectVPATargetMetricsWithMetrics(t *testing.T) {
	namespace := "default"
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace, Labels: map[string]string{"app": "api"}},
	}
	metrics := &metricsv1beta1.PodMetrics{
		TypeMeta:   metav1.TypeMeta{Kind: "PodMetrics", APIVersion: "metrics.k8s.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: namespace},
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			}},
		},
	}
	client := fake.NewSimpleClientset(deploy, pod)
	metricsClient := metricsfake.NewSimpleClientset(metrics)
	clients := &kube.Clients{Typed: client, Metrics: metricsClient}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	evidence, err := toolset.collectVPATargetMetrics(context.Background(), namespace, map[string]string{"kind": "Deployment", "name": "api"})
	if err != nil {
		t.Fatalf("collectVPATargetMetrics metrics: %v", err)
	}
	if evidence == nil || evidence["podMetrics"] == nil {
		t.Fatalf("expected pod metrics evidence")
	}
}
