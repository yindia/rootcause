package k8s

import (
	"context"
	"testing"
	"time"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
	"k8s.io/client-go/kubernetes/fake"
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

type metricsDiscovery struct{}

func (d *metricsDiscovery) ServerGroups() (*metav1.APIGroupList, error) {
	return &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "metrics.k8s.io"}}}, nil
}

func (d *metricsDiscovery) ServerResourcesForGroupVersion(string) (*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *metricsDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return nil, nil, nil
}

func (d *metricsDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *metricsDiscovery) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return nil, nil
}

func (d *metricsDiscovery) ServerVersion() (*version.Info, error) {
	return &version.Info{}, nil
}

func (d *metricsDiscovery) OpenAPISchema() (*openapi_v2.Document, error) {
	return nil, nil
}

func (d *metricsDiscovery) OpenAPIV3() openapi.Client {
	return nil
}

func (d *metricsDiscovery) RESTClient() rest.Interface { return nil }
func (d *metricsDiscovery) Fresh() bool               { return true }
func (d *metricsDiscovery) Invalidate()               {}
func (d *metricsDiscovery) WithLegacy() discovery.DiscoveryInterface {
	return d
}

func TestHandleResourceUsageMetrics(t *testing.T) {
	podMetric := &metricsv1beta1.PodMetrics{
		TypeMeta:  metav1.TypeMeta{Kind: "PodMetrics", APIVersion: "metrics.k8s.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Timestamp:  metav1.NewTime(time.Now()),
		Window:     metav1.Duration{Duration: time.Minute},
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			}},
		},
	}
	nodeMetric := &metricsv1beta1.NodeMetrics{
		TypeMeta: metav1.TypeMeta{Kind: "NodeMetrics", APIVersion: "metrics.k8s.io/v1beta1"},
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Timestamp:  metav1.NewTime(time.Now()),
		Window:     metav1.Duration{Duration: time.Minute},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	metricsClient := metricsfake.NewSimpleClientset(podMetric, nodeMetric)
	client := fake.NewSimpleClientset(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	clients := &kube.Clients{Typed: client, Metrics: metricsClient, Discovery: &metricsDiscovery{}}

	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})

	result, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{
		User: policy.User{Role: policy.RoleCluster},
		Arguments: map[string]any{
			"namespace":    "default",
			"includePods":  true,
			"includeNodes": true,
		},
	})
	if err != nil {
		t.Fatalf("handleResourceUsage: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["summary"] == nil {
		t.Fatalf("expected summary in result")
	}
}

func TestHandleResourceUsageNoMetricsClient(t *testing.T) {
	cfg := config.DefaultConfig()
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{
		Config:   &cfg,
		Clients:  &kube.Clients{},
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
	})
	_, err := toolset.handleResourceUsage(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}})
	if err == nil {
		t.Fatalf("expected error when metrics client missing")
	}
}

func TestSummarizeMetricHelpers(t *testing.T) {
	metric := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("5m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		}}},
	}
	usage := summarizePodMetric(metric)
	if usage.Name != "api" || usage.CPUMilli == 0 {
		t.Fatalf("unexpected pod usage: %#v", usage)
	}
	nodeMetric := &metricsv1beta1.NodeMetrics{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}, Usage: corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("200m"),
		corev1.ResourceMemory: resource.MustParse("2Gi"),
	}}
	nodeUsageResult := summarizeNodeMetric(nodeMetric)
	if nodeUsageResult.Name != "node-1" || nodeUsageResult.CPUMilli == 0 {
		t.Fatalf("unexpected node usage: %#v", nodeUsageResult)
	}

	podUsage := []podUsage{
		{Name: "b", CPUMilli: 5, MemoryBytes: 2},
		{Name: "a", CPUMilli: 10, MemoryBytes: 1},
	}
	sortPodUsage(podUsage, "cpu")
	if podUsage[0].Name != "a" {
		t.Fatalf("expected cpu sort")
	}
	sortPodUsage(podUsage, "memory")
	if podUsage[0].Name != "b" {
		t.Fatalf("expected memory sort")
	}

	nodeUsageList := []nodeUsage{{Name: "b", CPUMilli: 2, MemoryBytes: 5}, {Name: "a", CPUMilli: 10, MemoryBytes: 1}}
	sortNodeUsage(nodeUsageList, "cpu")
	if nodeUsageList[0].Name != "a" {
		t.Fatalf("expected node cpu sort")
	}
	sortNodeUsage(nodeUsageList, "memory")
	if nodeUsageList[0].Name != "b" {
		t.Fatalf("expected node memory sort")
	}
}
