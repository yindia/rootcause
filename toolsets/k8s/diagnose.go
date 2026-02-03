package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func (t *Toolset) handleDiagnose(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	keyword := strings.TrimSpace(toString(args["keyword"]))
	namespace := toString(args["namespace"])
	if keyword == "" {
		return errorResult(errors.New("keyword is required")), errors.New("keyword is required")
	}
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return errorResult(err), err
		}
	}

	namespaces, err := t.allowedNamespaces(ctx, req.User)
	if err != nil {
		return errorResult(err), err
	}
	if namespace != "" {
		namespaces = []string{namespace}
	}

	analysis := render.NewAnalysis()
	matchCount := 0
	for _, ns := range namespaces {
		pods, err := t.ctx.Clients.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorResult(err), err
		}
		for _, pod := range pods.Items {
			if !podMatchesKeyword(&pod, keyword) {
				continue
			}
			matchCount++
			analysis.AddEvidence(fmt.Sprintf("%s/%s", ns, pod.Name), t.ctx.Evidence.PodStatusSummary(&pod))
			analysis.AddResource(fmt.Sprintf("pods/%s/%s", ns, pod.Name))
			if hasCrashLoop(&pod) {
				analysis.AddCause("CrashLoopBackOff", fmt.Sprintf("Pod %s is crash looping", pod.Name), "high")
				analysis.AddNextCheck("Review container logs and recent changes")
			}
			reason, message := pendingReason(&pod)
			if pod.Status.Phase == corev1.PodPending && reason == "Unschedulable" {
				analysis.AddCause("Unschedulable pod", message, "high")
				analysis.AddNextCheck("Check node capacity and taints")
			}
			if obj, err := toUnstructured(&pod); err == nil {
				events, err := t.ctx.Evidence.EventsForObject(ctx, obj)
				if err == nil && len(events) > 0 {
					analysis.AddEvidence(fmt.Sprintf("%s/%s events", ns, pod.Name), events)
				}
			}
			if matchCount >= 10 {
				break
			}
		}
		if matchCount >= 10 {
			break
		}
	}

	if matchCount == 0 {
		analysis.AddEvidence("status", "no matching pods found")
		analysis.AddNextCheck("Verify namespace and keyword")
	}

	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func podMatchesKeyword(pod *corev1.Pod, keyword string) bool {
	if strings.Contains(pod.Name, keyword) {
		return true
	}
	for key, value := range pod.Labels {
		if strings.Contains(key, keyword) || strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func toUnstructured(pod *corev1.Pod) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: objMap}, nil
}
