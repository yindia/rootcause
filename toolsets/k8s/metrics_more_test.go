package k8s

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestHandleResourceUsageSortAndLimit(t *testing.T) {
	podMetric1 := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Timestamp:  metav1.NewTime(time.Now()),
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			}},
		},
	}
	podMetric2 := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "api-2", Namespace: "default"},
		Timestamp:  metav1.NewTime(time.Now()),
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			}},
		},
	}
	nodeMetric1 := &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	nodeMetric2 := &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	metricsClient := metricsfake.NewSimpleClientset(podMetric1, podMetric2, nodeMetric1, nodeMetric2)
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	result, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":    "default",
			"includePods":  true,
			"includeNodes": true,
			"sortBy":       "memory",
			"limit":        float64(1),
		},
	})
	if err != nil {
		t.Fatalf("handleResourceUsage sort/limit: %v", err)
	}
	data := result.Data.(map[string]any)
	if pods, ok := data["pods"].([]podUsage); ok && len(pods) > 1 {
		t.Fatalf("expected pods to be limited")
	}
	if nodes, ok := data["nodes"].([]nodeUsage); ok && len(nodes) > 1 {
		t.Fatalf("expected nodes to be limited")
	}
}

func TestHandleResourceUsageNoPodsNoNodes(t *testing.T) {
	metricsClient := metricsfake.NewSimpleClientset()
	client := fake.NewSimpleClientset()
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"includePods":  false,
			"includeNodes": false,
		},
	}); err != nil {
		t.Fatalf("handleResourceUsage no pods/nodes: %v", err)
	}
}

func TestHandleResourceUsagePodMetricsNotFound(t *testing.T) {
	metricsClient := metricsfake.NewSimpleClientset()
	metricsClient.PrependReactor("list", "pods", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "pods"}, "")
	})
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":   "default",
			"includePods": true,
		},
	}); err != nil {
		t.Fatalf("handleResourceUsage pod metrics not found: %v", err)
	}
}

func TestHandleResourceUsageDiscoveryError(t *testing.T) {
	metricsClient := metricsfake.NewSimpleClientset()
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{Metrics: metricsClient},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}); err == nil {
		t.Fatalf("expected discovery error")
	}
}

func TestHandleResourceUsageNodeMetricsError(t *testing.T) {
	metricsClient := metricsfake.NewSimpleClientset()
	metricsClient.PrependReactor("list", "nodes", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":    "default",
			"includePods":  true,
			"includeNodes": true,
		},
	}); err == nil {
		t.Fatalf("expected node metrics error")
	}
}

func TestHandleResourceUsageAllowedNamespaces(t *testing.T) {
	podMetric := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("5m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		}}},
	}
	metricsClient := metricsfake.NewSimpleClientset(podMetric)
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
	})

	if _, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User:      policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{"includePods": true},
	}); err != nil {
		t.Fatalf("handleResourceUsage allowed namespaces: %v", err)
	}
}
