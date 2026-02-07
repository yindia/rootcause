package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestMetricsHelperFunctions(t *testing.T) {
	metric := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Containers: []metricsv1beta1.ContainerMetrics{
			{Name: "app", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("10m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			}},
			{Name: "sidecar", Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("5m"),
				corev1.ResourceMemory: resource.MustParse("16Mi"),
			}},
		},
	}
	podSummary := summarizePodMetric(metric)
	if podSummary.Name != "api" || podSummary.CPUMilli == 0 {
		t.Fatalf("unexpected pod summary: %#v", podSummary)
	}

	nodeMetric := &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
	nodeSummary := summarizeNodeMetric(nodeMetric)
	if nodeSummary.Name != "node-1" || nodeSummary.CPUMilli == 0 {
		t.Fatalf("unexpected node summary: %#v", nodeSummary)
	}

	usage := []podUsage{
		{CPUMilli: 5, MemoryBytes: 100},
		{CPUMilli: 10, MemoryBytes: 50},
	}
	sortPodUsage(usage, "cpu")
	sortPodUsage(usage, "memory")

	nodes := []nodeUsage{
		{CPUMilli: 5, MemoryBytes: 100},
		{CPUMilli: 10, MemoryBytes: 50},
	}
	sortNodeUsage(nodes, "cpu")
	sortNodeUsage(nodes, "memory")

	if formatCPU(100) == "" || formatMemory(1024) == "" {
		t.Fatalf("expected formatted quantities")
	}
}
