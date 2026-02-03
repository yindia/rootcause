package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

func (t *Toolset) handleOverview(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
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
	podCounts := map[string]int{}
	crashloopPods := []string{}
	for _, ns := range namespaces {
		list, err := t.ctx.Clients.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorResult(err), err
		}
		for _, pod := range list.Items {
			podCounts[string(pod.Status.Phase)]++
			if hasCrashLoop(&pod) {
				crashloopPods = append(crashloopPods, fmt.Sprintf("%s/%s", ns, pod.Name))
			}
		}
	}
	analysis := render.NewAnalysis()
	analysis.AddEvidence("podPhaseCounts", podCounts)
	if len(crashloopPods) > 0 {
		analysis.AddCause("CrashLoopBackOff detected", "One or more pods are crash looping", "warning")
		analysis.AddEvidence("crashLoopPods", crashloopPods)
		analysis.AddNextCheck("Inspect container logs for crash loop pods")
	}
	if req.User.Role == policy.RoleCluster {
		nodes, err := t.ctx.Clients.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil {
			analysis.AddEvidence("nodeCount", len(nodes.Items))
		}
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleCrashloopDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	selector := toString(req.Arguments["labelSelector"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	list, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return errorResult(err), err
	}
	analysis := render.NewAnalysis()
	for _, pod := range list.Items {
		if !hasCrashLoop(&pod) {
			continue
		}
		analysis.AddCause("CrashLoopBackOff", fmt.Sprintf("Pod %s is crash looping", pod.Name), "high")
		analysis.AddEvidence(pod.Name, t.ctx.Evidence.PodStatusSummary(&pod))
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, pod.Name))
	}
	if len(analysis.LikelyRootCauses) == 0 {
		analysis.AddEvidence("status", "no crash loop pods found")
	}
	analysis.AddNextCheck("Review container logs and recent changes")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleSchedulingDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	selector := toString(req.Arguments["labelSelector"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	list, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return errorResult(err), err
	}
	resourceQuotas, quotaErr := t.ctx.Clients.Typed.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	limitRanges, limitErr := t.ctx.Clients.Typed.CoreV1().LimitRanges(namespace).List(ctx, metav1.ListOptions{})
	priorityClasses, priorityErr := t.ctx.Clients.Typed.SchedulingV1().PriorityClasses().List(ctx, metav1.ListOptions{})
	analysis := render.NewAnalysis()
	if quotaErr != nil {
		analysis.AddEvidence("resourceQuotaError", quotaErr.Error())
	} else if len(resourceQuotas.Items) > 0 {
		analysis.AddEvidence("resourceQuotas", summarizeResourceQuotas(resourceQuotas.Items))
		if exhausted := quotaExhaustedResources(resourceQuotas.Items); len(exhausted) > 0 {
			analysis.AddCause("ResourceQuota exhausted", strings.Join(exhausted, ", "), "high")
			analysis.AddNextCheck("Review ResourceQuota usage or request increases")
		}
	}
	if limitErr != nil {
		analysis.AddEvidence("limitRangeError", limitErr.Error())
	} else if len(limitRanges.Items) > 0 {
		analysis.AddEvidence("limitRanges", summarizeLimitRanges(limitRanges.Items))
		analysis.AddNextCheck("Verify pod requests/limits are within LimitRange bounds")
	}
	for _, pod := range list.Items {
		if pod.Status.Phase != corev1.PodPending {
			continue
		}
		reason, message := pendingReason(&pod)
		podEvidence := map[string]any{"reason": reason, "message": message}
		if pod.Spec.PriorityClassName != "" {
			podEvidence["priorityClass"] = pod.Spec.PriorityClassName
		}
		if pod.Spec.PreemptionPolicy != nil {
			podEvidence["preemptionPolicy"] = string(*pod.Spec.PreemptionPolicy)
		}
		if pod.Status.NominatedNodeName != "" {
			podEvidence["nominatedNode"] = pod.Status.NominatedNodeName
		}
		if priorityErr == nil {
			if pc := findPriorityClass(priorityClasses.Items, pod.Spec.PriorityClassName); pc != nil {
				podEvidence["priorityValue"] = pc.Value
			}
		}
		if events, err := podEvents(ctx, t, &pod); err == nil && len(events) > 0 {
			podEvidence["events"] = summarizeEvents(events)
			if hasPreemptionEvent(events) {
				analysis.AddCause("Preemption attempted", fmt.Sprintf("Pod %s triggered preemption", pod.Name), "medium")
			}
			if hasQuotaEvent(events) {
				analysis.AddCause("Quota prevents scheduling", fmt.Sprintf("Pod %s exceeded quota", pod.Name), "high")
			}
		}
		analysis.AddEvidence(pod.Name, podEvidence)
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, pod.Name))
		if reason == "Unschedulable" {
			analysis.AddCause("Unschedulable pod", message, "high")
		}
	}
	if len(analysis.Evidence) == 0 {
		analysis.AddEvidence("status", "no pending pods found")
	}
	analysis.AddNextCheck("Check node capacity and taints")
	if priorityErr != nil {
		analysis.AddEvidence("priorityClassError", priorityErr.Error())
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleHPADebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	name := toString(req.Arguments["name"])
	if namespace == "" {
		return errorResult(errors.New("namespace is required")), errors.New("namespace is required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	var hpas []autoscalingv2.HorizontalPodAutoscaler
	if name != "" {
		hpa, err := t.ctx.Clients.Typed.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errorResult(err), err
		}
		hpas = append(hpas, *hpa)
	} else {
		list, err := t.ctx.Clients.Typed.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return errorResult(err), err
		}
		hpas = append(hpas, list.Items...)
	}
	analysis := render.NewAnalysis()
	for _, hpa := range hpas {
		analysis.AddEvidence(hpa.Name, map[string]any{"currentReplicas": hpa.Status.CurrentReplicas, "desiredReplicas": hpa.Status.DesiredReplicas, "conditions": hpa.Status.Conditions})
		analysis.AddResource(fmt.Sprintf("hpas/%s/%s", namespace, hpa.Name))
		for _, cond := range hpa.Status.Conditions {
			if cond.Status == corev1.ConditionFalse {
				analysis.AddCause("HPA condition false", fmt.Sprintf("%s: %s", cond.Type, cond.Message), "medium")
			}
		}
	}
	if len(hpas) == 0 {
		analysis.AddEvidence("status", "no hpas found")
	}
	analysis.AddNextCheck("Verify metrics server and target deployment")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleNetworkDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	serviceName := toString(req.Arguments["service"])
	if namespace == "" || serviceName == "" {
		return errorResult(errors.New("namespace and service are required")), errors.New("namespace and service are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	service, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return errorResult(err), err
	}
	analysis := render.NewAnalysis()
	analysis.AddResource(fmt.Sprintf("services/%s/%s", namespace, serviceName))
	selector := labels.Set(service.Spec.Selector).AsSelector()
	pods, err := t.ctx.Evidence.RelatedPods(ctx, namespace, selector)
	if err == nil {
		var readyPods []string
		for _, pod := range pods {
			if isPodReady(&pod) {
				readyPods = append(readyPods, pod.Name)
			}
		}
		analysis.AddEvidence("readyPods", readyPods)
	}
	endpoints, err := t.ctx.Evidence.EndpointsForService(ctx, namespace, serviceName)
	if err == nil {
		endpointCount := 0
		for _, subset := range endpoints.Subsets {
			endpointCount += len(subset.Addresses)
		}
		analysis.AddEvidence("endpointCount", endpointCount)
		if endpointCount == 0 {
			analysis.AddCause("No ready endpoints", "Service has no ready endpoints", "high")
		}
	}
	policies, netpolErr := t.ctx.Clients.Typed.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if netpolErr != nil {
		analysis.AddEvidence("networkPolicyError", netpolErr.Error())
	} else if len(policies.Items) == 0 {
		analysis.AddEvidence("networkPolicies", "no network policies found")
	} else {
		analysis.AddEvidence("networkPolicies", summarizeNetworkPolicies(policies.Items))
		if err == nil {
			for _, pod := range pods {
				selected := policiesSelectingPod(policies.Items, &pod)
				if len(selected) == 0 {
					continue
				}
				ingressBlocked, egressBlocked := networkPolicyBlockStatus(selected)
				analysis.AddEvidence(fmt.Sprintf("networkPolicy.%s", pod.Name), map[string]any{
					"policies":       policyNames(selected),
					"ingressBlocked": ingressBlocked,
					"egressBlocked":  egressBlocked,
					"pod":            pod.Name,
				})
				if ingressBlocked {
					analysis.AddCause("NetworkPolicy blocks ingress", fmt.Sprintf("Pod %s has no allowed ingress rules", pod.Name), "high")
				}
				if egressBlocked {
					analysis.AddCause("NetworkPolicy blocks egress", fmt.Sprintf("Pod %s has no allowed egress rules", pod.Name), "high")
				}
			}
		}
		analysis.AddNextCheck("Review NetworkPolicy selectors and allow rules for service pods")
	}
	analysis.AddNextCheck("Check service selectors and pod readiness")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handlePrivateLinkDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	serviceName := toString(req.Arguments["service"])
	if namespace == "" || serviceName == "" {
		return errorResult(errors.New("namespace and service are required")), errors.New("namespace and service are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	service, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return errorResult(err), err
	}
	analysis := render.NewAnalysis()
	analysis.AddResource(fmt.Sprintf("services/%s/%s", namespace, serviceName))
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		analysis.AddCause("Service not LoadBalancer", "PrivateLink typically uses LoadBalancer services", "medium")
	}
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		analysis.AddCause("No load balancer ingress", "Service does not have an external ingress yet", "high")
		analysis.AddNextCheck("Check cloud provider LB provisioning and service annotations")
	} else {
		analysis.AddEvidence("ingress", service.Status.LoadBalancer.Ingress)
	}
	analysis.AddNextCheck("Verify private link endpoint policies and security groups")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func hasCrashLoop(pod *corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && (status.State.Waiting.Reason == "CrashLoopBackOff" || status.State.Waiting.Reason == "ImagePullBackOff" || status.State.Waiting.Reason == "ErrImagePull") {
			return true
		}
	}
	return false
}

func pendingReason(pod *corev1.Pod) (string, string) {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			return string(condition.Reason), condition.Message
		}
	}
	return "Pending", pod.Status.Message
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func summarizeResourceQuotas(quotas []corev1.ResourceQuota) []map[string]any {
	out := make([]map[string]any, 0, len(quotas))
	for i := range quotas {
		quota := quotas[i]
		out = append(out, map[string]any{
			"name": quota.Name,
			"hard": quota.Status.Hard,
			"used": quota.Status.Used,
		})
	}
	return out
}

func quotaExhaustedResources(quotas []corev1.ResourceQuota) []string {
	found := map[string]struct{}{}
	for i := range quotas {
		quota := quotas[i]
		for resourceName, hard := range quota.Status.Hard {
			used, ok := quota.Status.Used[resourceName]
			if !ok {
				continue
			}
			if used.Cmp(hard) >= 0 {
				found[fmt.Sprintf("%s:%s", quota.Name, resourceName)] = struct{}{}
			}
		}
	}
	var out []string
	for value := range found {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func summarizeLimitRanges(ranges []corev1.LimitRange) []map[string]any {
	out := make([]map[string]any, 0, len(ranges))
	for i := range ranges {
		lr := ranges[i]
		limits := []map[string]any{}
		for _, limit := range lr.Spec.Limits {
			item := map[string]any{"type": limit.Type}
			if len(limit.Min) > 0 {
				item["min"] = limit.Min
			}
			if len(limit.Max) > 0 {
				item["max"] = limit.Max
			}
			if len(limit.Default) > 0 {
				item["default"] = limit.Default
			}
			if len(limit.DefaultRequest) > 0 {
				item["defaultRequest"] = limit.DefaultRequest
			}
			limits = append(limits, item)
		}
		out = append(out, map[string]any{"name": lr.Name, "limits": limits})
	}
	return out
}

func findPriorityClass(classes []schedulingv1.PriorityClass, name string) *schedulingv1.PriorityClass {
	if name == "" {
		return nil
	}
	for i := range classes {
		if classes[i].Name == name {
			return &classes[i]
		}
	}
	return nil
}

func podEvents(ctx context.Context, t *Toolset, pod *corev1.Pod) ([]corev1.Event, error) {
	if pod == nil || pod.UID == "" || pod.Namespace == "" {
		return nil, nil
	}
	list, err := t.ctx.Clients.Typed.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.uid=%s", pod.UID),
	})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func summarizeEvents(events []corev1.Event) []map[string]any {
	out := []map[string]any{}
	for _, event := range events {
		out = append(out, map[string]any{
			"reason":  event.Reason,
			"message": event.Message,
			"type":    event.Type,
		})
	}
	return out
}

func hasPreemptionEvent(events []corev1.Event) bool {
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Reason), "preempt") {
			return true
		}
		if strings.Contains(strings.ToLower(event.Message), "preempt") {
			return true
		}
	}
	return false
}

func hasQuotaEvent(events []corev1.Event) bool {
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Message), "exceeded quota") {
			return true
		}
	}
	return false
}

func summarizeNetworkPolicies(policies []networkingv1.NetworkPolicy) []map[string]any {
	out := make([]map[string]any, 0, len(policies))
	for i := range policies {
		policy := policies[i]
		out = append(out, map[string]any{
			"name":        policy.Name,
			"podSelector": policy.Spec.PodSelector.MatchLabels,
			"policyTypes": policy.Spec.PolicyTypes,
			"ingress":     len(policy.Spec.Ingress),
			"egress":      len(policy.Spec.Egress),
		})
	}
	return out
}

func policiesSelectingPod(policies []networkingv1.NetworkPolicy, pod *corev1.Pod) []networkingv1.NetworkPolicy {
	if pod == nil {
		return nil
	}
	var out []networkingv1.NetworkPolicy
	for i := range policies {
		policy := policies[i]
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			out = append(out, policy)
		}
	}
	return out
}

func networkPolicyBlockStatus(policies []networkingv1.NetworkPolicy) (bool, bool) {
	ingressApplies := false
	ingressAllowed := false
	egressApplies := false
	egressAllowed := false
	for i := range policies {
		policy := policies[i]
		if policyAppliesIngress(&policy) {
			ingressApplies = true
			if len(policy.Spec.Ingress) > 0 {
				ingressAllowed = true
			}
		}
		if policyAppliesEgress(&policy) {
			egressApplies = true
			if len(policy.Spec.Egress) > 0 {
				egressAllowed = true
			}
		}
	}
	return ingressApplies && !ingressAllowed, egressApplies && !egressAllowed
}

func policyNames(policies []networkingv1.NetworkPolicy) []string {
	out := make([]string, 0, len(policies))
	for i := range policies {
		out = append(out, policies[i].Name)
	}
	sort.Strings(out)
	return out
}
