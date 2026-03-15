package k8s

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type nodeUtilization struct {
	Node                   string
	CPURequestedMilli      int64
	CPUAllocatableMilli    int64
	MemoryRequestedBytes   int64
	MemoryAllocatableBytes int64
	CPUPct                 float64
	MemoryPct              float64
}

func (t *Toolset) nodeUtilizationEvidence(ctx context.Context) ([]map[string]any, string) {
	if t == nil || t.ctx.Clients == nil || t.ctx.Clients.Typed == nil {
		return nil, "missing kube clients"
	}
	nodes, err := t.ctx.Clients.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err.Error()
	}
	pods, err := t.ctx.Clients.Typed.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err.Error()
	}

	nodeUsage := map[string]*nodeUtilization{}
	for _, node := range nodes.Items {
		nodeUsage[node.Name] = &nodeUtilization{
			Node:                   node.Name,
			CPUAllocatableMilli:    quantityMilli(node.Status.Allocatable[corev1.ResourceCPU]),
			MemoryAllocatableBytes: quantityValue(node.Status.Allocatable[corev1.ResourceMemory]),
		}
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == "" {
			continue
		}
		entry, ok := nodeUsage[pod.Spec.NodeName]
		if !ok {
			continue
		}
		reqCPU, reqMem := podResourceRequests(pod)
		entry.CPURequestedMilli += reqCPU
		entry.MemoryRequestedBytes += reqMem
	}

	var summaries []nodeUtilization
	for _, entry := range nodeUsage {
		if entry.CPUAllocatableMilli > 0 {
			entry.CPUPct = float64(entry.CPURequestedMilli) / float64(entry.CPUAllocatableMilli)
		}
		if entry.MemoryAllocatableBytes > 0 {
			entry.MemoryPct = float64(entry.MemoryRequestedBytes) / float64(entry.MemoryAllocatableBytes)
		}
		summaries = append(summaries, *entry)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CPUPct > summaries[j].CPUPct
	})

	var out []map[string]any
	for _, entry := range summaries {
		out = append(out, map[string]any{
			"node":                   entry.Node,
			"cpuRequestedMilli":      entry.CPURequestedMilli,
			"cpuAllocatableMilli":    entry.CPUAllocatableMilli,
			"cpuPct":                 entry.CPUPct,
			"memoryRequestedBytes":   entry.MemoryRequestedBytes,
			"memoryAllocatableBytes": entry.MemoryAllocatableBytes,
			"memoryPct":              entry.MemoryPct,
		})
	}
	return out, ""
}

func nodesOverutilized(entries []map[string]any, threshold float64) bool {
	for _, entry := range entries {
		if pct, ok := entry["cpuPct"].(float64); ok && pct >= threshold {
			return true
		}
		if pct, ok := entry["memoryPct"].(float64); ok && pct >= threshold {
			return true
		}
	}
	return false
}

func podResourceRequests(pod corev1.Pod) (int64, int64) {
	var cpuMilli int64
	var memBytes int64
	for _, container := range pod.Spec.Containers {
		cpuMilli += quantityMilli(container.Resources.Requests[corev1.ResourceCPU])
		memBytes += quantityValue(container.Resources.Requests[corev1.ResourceMemory])
	}
	var initCPU int64
	var initMem int64
	for _, container := range pod.Spec.InitContainers {
		reqCPU := quantityMilli(container.Resources.Requests[corev1.ResourceCPU])
		reqMem := quantityValue(container.Resources.Requests[corev1.ResourceMemory])
		if reqCPU > initCPU {
			initCPU = reqCPU
		}
		if reqMem > initMem {
			initMem = reqMem
		}
	}
	cpuMilli += initCPU
	memBytes += initMem
	return cpuMilli, memBytes
}

func quantityMilli(q resource.Quantity) int64 {
	if q.IsZero() {
		return 0
	}
	return q.MilliValue()
}

func quantityValue(q resource.Quantity) int64 {
	if q.IsZero() {
		return 0
	}
	return q.Value()
}
