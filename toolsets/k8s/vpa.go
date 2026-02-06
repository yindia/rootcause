package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func (t *Toolset) handleVPADebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	name := toString(req.Arguments["name"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	analysis := render.NewAnalysis()
	present, groups, err := kube.GroupsPresent(t.ctx.Clients.Discovery, []string{"autoscaling.k8s.io"})
	if err != nil {
		return errorResult(err), err
	}
	if !present {
		analysis.AddEvidence("status", "vpa not detected")
		analysis.AddEvidence("groupsChecked", []string{"autoscaling.k8s.io"})
		analysis.AddNextCheck("Install Vertical Pod Autoscaler CRDs")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}

	gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "VerticalPodAutoscaler", "", "autoscaling.k8s.io")
	if err != nil {
		return errorResult(err), err
	}
	if !namespaced {
		return errorResult(errors.New("vpa resource is not namespaced")), errors.New("vpa resource is not namespaced")
	}

	var items []unstructured.Unstructured
	if name != "" {
		obj, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				analysis.AddEvidence("status", "vpa not found")
				analysis.AddNextCheck("Verify VPA name and namespace")
				return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
			}
			return errorResult(err), err
		}
		items = append(items, *obj)
	} else {
		list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorResult(err), err
		}
		items = append(items, list.Items...)
	}

	for i := range items {
		obj := &items[i]
		targetRef := extractVPATargetRef(obj)
		updateMode := nestedString(obj, "spec", "updatePolicy", "updateMode")
		conditions := extractVPAConditions(obj)
		recommendations := extractVPARecommendations(obj)
		vpaEvidence := map[string]any{
			"targetRef":       targetRef,
			"updateMode":      updateMode,
			"conditions":      conditions,
			"recommendations": recommendations,
		}
		if updateMode == "" {
			updateMode = "Auto"
		}
		if strings.EqualFold(updateMode, "Off") {
			analysis.AddCause("VPA updates disabled", fmt.Sprintf("%s updateMode is Off", obj.GetName()), "medium")
		}
		if len(recommendations) == 0 {
			analysis.AddCause("No VPA recommendation", fmt.Sprintf("%s has no recommendations", obj.GetName()), "low")
		}

		workloadEvidence, err := t.collectVPATargetMetrics(ctx, namespace, targetRef)
		if err != nil {
			vpaEvidence["workloadLookup"] = err.Error()
		} else if workloadEvidence != nil {
			vpaEvidence["workload"] = workloadEvidence
		}

		analysis.AddEvidence(obj.GetName(), vpaEvidence)
		analysis.AddResource(t.ctx.Evidence.ResourceRef(gvr, namespace, obj.GetName()))
	}

	if len(items) == 0 {
		analysis.AddEvidence("status", "no vpa resources found")
	}
	analysis.AddNextCheck("Verify VPA admission controller and metrics-server")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func extractVPATargetRef(obj *unstructured.Unstructured) map[string]string {
	ref := map[string]string{}
	if obj == nil {
		return ref
	}
	if target, ok := nestedMap(obj, "spec", "targetRef"); ok {
		for _, key := range []string{"apiVersion", "kind", "name"} {
			if value, ok := target[key]; ok {
				ref[key] = fmt.Sprintf("%v", value)
			}
		}
	}
	return ref
}

func extractVPAConditions(obj *unstructured.Unstructured) []map[string]any {
	items, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		cond, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"type":               cond["type"],
			"status":             cond["status"],
			"reason":             cond["reason"],
			"message":            cond["message"],
			"lastTransitionTime": cond["lastTransitionTime"],
		})
	}
	return out
}

func extractVPARecommendations(obj *unstructured.Unstructured) []map[string]any {
	items, _, _ := unstructured.NestedSlice(obj.Object, "status", "recommendation", "containerRecommendations")
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		rec, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, map[string]any{
			"container":   rec["containerName"],
			"lowerBound":  stringifyResourceMap(rec["lowerBound"]),
			"target":      stringifyResourceMap(rec["target"]),
			"upperBound":  stringifyResourceMap(rec["upperBound"]),
			"uncapped":    stringifyResourceMap(rec["uncappedTarget"]),
			"confidence":  rec["confidence"],
			"annotations": rec["annotations"],
		})
	}
	return out
}

func (t *Toolset) collectVPATargetMetrics(ctx context.Context, namespace string, targetRef map[string]string) (map[string]any, error) {
	if targetRef == nil {
		return nil, nil
	}
	name := targetRef["name"]
	kind := targetRef["kind"]
	if name == "" || kind == "" {
		return nil, nil
	}

	selector, workloadRef, err := t.selectorForTarget(ctx, namespace, kind, name)
	if err != nil {
		return nil, err
	}
	if selector == nil {
		return nil, nil
	}

	pods, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	evidence := map[string]any{
		"workload": workloadRef,
		"pods":     len(pods.Items),
	}

	if t.ctx.Clients.Metrics == nil {
		evidence["metrics"] = "metrics client not available"
		return evidence, nil
	}
	podMetrics, err := t.ctx.Clients.Metrics.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			evidence["metrics"] = "metrics.k8s.io not available"
			return evidence, nil
		}
		return nil, err
	}

	metricsByPod := map[string]*metricsv1beta1.PodMetrics{}
	for i := range podMetrics.Items {
		item := podMetrics.Items[i]
		metricsByPod[item.Name] = &item
	}

	var totalCPU, totalMem int64
	type podMetricSummary struct {
		Name        string `json:"name"`
		CPU         string `json:"cpu"`
		Memory      string `json:"memory"`
		CPUMilli    int64  `json:"-"`
		MemoryBytes int64  `json:"-"`
	}
	var summaries []podMetricSummary
	for _, pod := range pods.Items {
		metric := metricsByPod[pod.Name]
		if metric == nil {
			continue
		}
		cpuMilli, memBytes := sumPodMetrics(metric)
		totalCPU += cpuMilli
		totalMem += memBytes
		summaries = append(summaries, podMetricSummary{
			Name:        pod.Name,
			CPU:         formatCPU(cpuMilli),
			Memory:      formatMemory(memBytes),
			CPUMilli:    cpuMilli,
			MemoryBytes: memBytes,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CPUMilli > summaries[j].CPUMilli
	})
	if len(summaries) > 10 {
		summaries = summaries[:10]
	}
	evidence["podMetrics"] = summaries
	evidence["totalCPU"] = formatCPU(totalCPU)
	evidence["totalMemory"] = formatMemory(totalMem)
	return evidence, nil
}

func sumPodMetrics(metric *metricsv1beta1.PodMetrics) (int64, int64) {
	var cpuMilli int64
	var memBytes int64
	for _, container := range metric.Containers {
		cpuQty := container.Usage[corev1.ResourceCPU]
		memQty := container.Usage[corev1.ResourceMemory]
		cpuMilli += cpuQty.MilliValue()
		memBytes += memQty.Value()
	}
	return cpuMilli, memBytes
}

func (t *Toolset) selectorForTarget(ctx context.Context, namespace, kind, name string) (labels.Selector, map[string]any, error) {
	kind = strings.ToLower(kind)
	switch kind {
	case "deployment":
		deploy, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
		if err != nil {
			return nil, nil, err
		}
		return selector, map[string]any{"kind": "Deployment", "name": name}, nil
	case "statefulset":
		sts, err := t.ctx.Clients.Typed.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
		if err != nil {
			return nil, nil, err
		}
		return selector, map[string]any{"kind": "StatefulSet", "name": name}, nil
	case "daemonset":
		ds, err := t.ctx.Clients.Typed.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
		if err != nil {
			return nil, nil, err
		}
		return selector, map[string]any{"kind": "DaemonSet", "name": name}, nil
	case "replicaset":
		rs, err := t.ctx.Clients.Typed.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
		if err != nil {
			return nil, nil, err
		}
		return selector, map[string]any{"kind": "ReplicaSet", "name": name}, nil
	case "replicationcontroller":
		rc, err := t.ctx.Clients.Typed.CoreV1().ReplicationControllers(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		selector := labels.SelectorFromSet(rc.Spec.Selector)
		return selector, map[string]any{"kind": "ReplicationController", "name": name}, nil
	default:
		return nil, map[string]any{"kind": kind, "name": name}, nil
	}
}

func nestedMap(obj *unstructured.Unstructured, fields ...string) (map[string]any, bool) {
	if obj == nil {
		return nil, false
	}
	value, ok, _ := unstructured.NestedMap(obj.Object, fields...)
	return value, ok
}

func stringifyResourceMap(value any) map[string]string {
	out := map[string]string{}
	if value == nil {
		return out
	}
	input, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for k, v := range input {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}
