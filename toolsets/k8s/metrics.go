package k8s

import (
	"context"
	"errors"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type podUsage struct {
	Namespace   string            `json:"namespace"`
	Name        string            `json:"name"`
	Node        string            `json:"node,omitempty"`
	CPU         string            `json:"cpu"`
	Memory      string            `json:"memory"`
	CPUMilli    int64             `json:"cpuMilli"`
	MemoryBytes int64             `json:"memoryBytes"`
	Containers  []containerUsage  `json:"containers,omitempty"`
	Window      string            `json:"window,omitempty"`
	Timestamp   string            `json:"timestamp,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type containerUsage struct {
	Name        string `json:"name"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	CPUMilli    int64  `json:"cpuMilli"`
	MemoryBytes int64  `json:"memoryBytes"`
}

type nodeUsage struct {
	Name        string `json:"name"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	CPUMilli    int64  `json:"cpuMilli"`
	MemoryBytes int64  `json:"memoryBytes"`
	Window      string `json:"window,omitempty"`
	Timestamp   string `json:"timestamp,omitempty"`
}

func (t *Toolset) handleResourceUsage(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if t.ctx.Clients == nil || t.ctx.Clients.Metrics == nil {
		return errorResult(errors.New("metrics client not available")), errors.New("metrics client not available")
	}
	present, _, err := kube.GroupsPresent(t.ctx.Clients.Discovery, []string{"metrics.k8s.io"})
	if err != nil {
		return errorResult(err), err
	}
	if !present {
		return mcp.ToolResult{Data: map[string]any{
			"error": "metrics.k8s.io API not detected; install metrics-server",
		}}, nil
	}

	args := req.Arguments
	namespace := toString(args["namespace"])
	includePods := toBool(args["includePods"], true)
	includeNodes := toBool(args["includeNodes"], true)
	sortBy := strings.ToLower(toString(args["sortBy"]))
	limit := toInt(args["limit"], 0)
	if sortBy != "memory" {
		sortBy = "cpu"
	}

	warnings := []string{}
	if includeNodes && req.User.Role != policy.RoleCluster {
		includeNodes = false
		warnings = append(warnings, "node metrics require cluster role")
	}

	result := map[string]any{}
	var namespaces []string
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return errorResult(err), err
		}
		namespaces = []string{namespace}
	} else if includePods {
		namespaces, err = t.allowedNamespaces(ctx, req.User)
		if err != nil {
			return errorResult(err), err
		}
	}

	var podMetrics []podUsage
	var totalPodCPU, totalPodMem int64
	if includePods {
		for _, ns := range namespaces {
			list, err := t.ctx.Clients.Metrics.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return errorResult(err), err
			}
			for i := range list.Items {
				usage := summarizePodMetric(&list.Items[i])
				totalPodCPU += usage.CPUMilli
				totalPodMem += usage.MemoryBytes
				podMetrics = append(podMetrics, usage)
			}
		}
		sortPodUsage(podMetrics, sortBy)
		if limit > 0 && len(podMetrics) > limit {
			podMetrics = podMetrics[:limit]
		}
		result["pods"] = podMetrics
	}

	var nodeMetrics []nodeUsage
	var totalNodeCPU, totalNodeMem int64
	if includeNodes {
		list, err := t.ctx.Clients.Metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, "node metrics not available")
			} else {
				return errorResult(err), err
			}
		} else {
			for i := range list.Items {
				usage := summarizeNodeMetric(&list.Items[i])
				totalNodeCPU += usage.CPUMilli
				totalNodeMem += usage.MemoryBytes
				nodeMetrics = append(nodeMetrics, usage)
			}
			sortNodeUsage(nodeMetrics, sortBy)
			if limit > 0 && len(nodeMetrics) > limit {
				nodeMetrics = nodeMetrics[:limit]
			}
			result["nodes"] = nodeMetrics
		}
	}

	summary := map[string]any{
		"podCount":     len(podMetrics),
		"nodeCount":    len(nodeMetrics),
		"totalPodCPU":  formatCPU(totalPodCPU),
		"totalPodMem":  formatMemory(totalPodMem),
		"totalNodeCPU": formatCPU(totalNodeCPU),
		"totalNodeMem": formatMemory(totalNodeMem),
	}
	result["summary"] = summary
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}
	return mcp.ToolResult{Data: result, Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func summarizePodMetric(metric *metricsv1beta1.PodMetrics) podUsage {
	var totalCPU, totalMem int64
	var containers []containerUsage
	for _, container := range metric.Containers {
		cpuQty := container.Usage[corev1.ResourceCPU]
		memQty := container.Usage[corev1.ResourceMemory]
		cpuMilli := cpuQty.MilliValue()
		memBytes := memQty.Value()
		totalCPU += cpuMilli
		totalMem += memBytes
		containers = append(containers, containerUsage{
			Name:        container.Name,
			CPU:         cpuQty.String(),
			Memory:      memQty.String(),
			CPUMilli:    cpuMilli,
			MemoryBytes: memBytes,
		})
	}
	return podUsage{
		Namespace:   metric.Namespace,
		Name:        metric.Name,
		CPU:         formatCPU(totalCPU),
		Memory:      formatMemory(totalMem),
		CPUMilli:    totalCPU,
		MemoryBytes: totalMem,
		Containers:  containers,
		Window:      metric.Window.String(),
		Timestamp:   metric.Timestamp.String(),
		Labels:      metric.Labels,
	}
}

func summarizeNodeMetric(metric *metricsv1beta1.NodeMetrics) nodeUsage {
	cpuQty := metric.Usage[corev1.ResourceCPU]
	memQty := metric.Usage[corev1.ResourceMemory]
	return nodeUsage{
		Name:        metric.Name,
		CPU:         cpuQty.String(),
		Memory:      memQty.String(),
		CPUMilli:    cpuQty.MilliValue(),
		MemoryBytes: memQty.Value(),
		Window:      metric.Window.String(),
		Timestamp:   metric.Timestamp.String(),
	}
}

func sortPodUsage(items []podUsage, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		if sortBy == "memory" {
			return items[i].MemoryBytes > items[j].MemoryBytes
		}
		return items[i].CPUMilli > items[j].CPUMilli
	})
}

func sortNodeUsage(items []nodeUsage, sortBy string) {
	sort.Slice(items, func(i, j int) bool {
		if sortBy == "memory" {
			return items[i].MemoryBytes > items[j].MemoryBytes
		}
		return items[i].CPUMilli > items[j].CPUMilli
	})
}

func formatCPU(milli int64) string {
	q := resource.NewMilliQuantity(milli, resource.DecimalSI)
	return q.String()
}

func formatMemory(bytes int64) string {
	q := resource.NewQuantity(bytes, resource.BinarySI)
	return q.String()
}

func toBool(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func toInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return fallback
}
