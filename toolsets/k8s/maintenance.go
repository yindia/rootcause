package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"rootcause/internal/mcp"
)

var defaultCleanupStates = []string{
	"Evicted",
	"ContainerStatusUnknown",
	"Completed",
	"Error",
	"ImagePullBackOff",
	"CrashLoopBackOff",
}

func (t *Toolset) handleCleanupPods(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	states := toStringSlice(args["states"])
	if len(states) == 0 {
		states = append([]string{}, defaultCleanupStates...)
	}
	stateSet := map[string]struct{}{}
	for _, state := range states {
		stateSet[state] = struct{}{}
	}
	labelSelector := toString(args["labelSelector"])
	pods, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return errorResult(err), err
	}

	var deleted []map[string]any
	for _, pod := range pods.Items {
		problemStates := podProblemStates(&pod)
		if !intersects(problemStates, stateSet) {
			continue
		}
		err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return errorResult(err), err
		}
		deleted = append(deleted, map[string]any{"pod": pod.Name, "states": problemStates})
	}

	return mcp.ToolResult{Data: map[string]any{"deleted": deleted}, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleNodeManagement(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
		return errorResult(err), err
	}
	args := req.Arguments
	action := strings.ToLower(toString(args["action"]))
	nodeName := toString(args["nodeName"])
	if action == "" || nodeName == "" {
		return errorResult(errors.New("action and nodeName are required")), errors.New("action and nodeName are required")
	}
	graceSeconds := int64(30)
	if val, ok := args["gracePeriodSeconds"].(float64); ok {
		graceSeconds = int64(val)
	}
	force := false
	if val, ok := args["force"].(bool); ok {
		force = val
	}

	switch action {
	case "cordon":
		return t.patchNodeUnschedulable(ctx, nodeName, true)
	case "uncordon":
		return t.patchNodeUnschedulable(ctx, nodeName, false)
	case "drain":
		return t.drainNode(ctx, nodeName, graceSeconds, force)
	default:
		return errorResult(errors.New("unsupported node management action")), errors.New("unsupported node management action")
	}
}

func (t *Toolset) patchNodeUnschedulable(ctx context.Context, nodeName string, unschedulable bool) (mcp.ToolResult, error) {
	patch := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable)
	obj, err := t.ctx.Clients.Typed.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{"node": obj.Name, "unschedulable": unschedulable}}, nil
}

func (t *Toolset) drainNode(ctx context.Context, nodeName string, graceSeconds int64, force bool) (mcp.ToolResult, error) {
	_, err := t.patchNodeUnschedulable(ctx, nodeName, true)
	if err != nil {
		return errorResult(err), err
	}

	pods, err := t.ctx.Clients.Typed.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName)})
	if err != nil {
		return errorResult(err), err
	}

	var evicted []string
	var skipped []string
	var failed []string

	for _, pod := range pods.Items {
		if isMirrorPod(&pod) || isDaemonSetPod(&pod) {
			skipped = append(skipped, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			continue
		}
		eviction := &policyv1.Eviction{
			ObjectMeta:    metav1.ObjectMeta{Name: pod.Name, Namespace: pod.Namespace},
			DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &graceSeconds},
		}
		err := t.ctx.Clients.Typed.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction)
		if err != nil && force {
			err = t.ctx.Clients.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &graceSeconds})
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s/%s: %v", pod.Namespace, pod.Name, err))
			continue
		}
		evicted = append(evicted, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
	}

	return mcp.ToolResult{Data: map[string]any{
		"node":      nodeName,
		"evicted":   evicted,
		"skipped":   skipped,
		"failed":    failed,
		"forced":    force,
		"timestamp": time.Now().UTC(),
	}}, nil
}

func podProblemStates(pod *corev1.Pod) []string {
	states := map[string]struct{}{}
	if pod.Status.Reason == "Evicted" {
		states["Evicted"] = struct{}{}
	}
	if pod.Status.Phase == corev1.PodSucceeded {
		states["Completed"] = struct{}{}
	}
	if pod.Status.Phase == corev1.PodFailed {
		states["Error"] = struct{}{}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			reason := status.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ContainerStatusUnknown":
				states[reason] = struct{}{}
			}
		}
	}
	var list []string
	for state := range states {
		list = append(list, state)
	}
	return list
}

func intersects(problemStates []string, target map[string]struct{}) bool {
	for _, state := range problemStates {
		if _, ok := target[state]; ok {
			return true
		}
	}
	return false
}

func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

func isMirrorPod(pod *corev1.Pod) bool {
	if pod.Annotations == nil {
		return false
	}
	_, ok := pod.Annotations[corev1.MirrorPodAnnotationKey]
	return ok
}
