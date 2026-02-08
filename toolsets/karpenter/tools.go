package karpenter

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

const karpenterNamespace = "karpenter"
const karpenterSelector = "app.kubernetes.io/name=karpenter"

var karpenterGroups = []string{"karpenter.sh"}

func (t *Toolset) handleStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, namespaces, groups, err := t.detectKarpenter(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "karpenter not detected")
		analysis.AddNextCheck("Install Karpenter or verify namespace")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}
	if len(namespaces) == 0 {
		namespaces = []string{karpenterNamespace}
		analysis.AddEvidence("namespaceFallback", karpenterNamespace)
	}
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		deployments, err := t.ctx.Clients.Typed.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: karpenterSelector})
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for _, deploy := range deployments.Items {
			analysis.AddEvidence(deploy.Name, map[string]any{"ready": deploy.Status.ReadyReplicas, "desired": *deploy.Spec.Replicas})
			if deploy.Status.ReadyReplicas < *deploy.Spec.Replicas {
				analysis.AddCause("Karpenter not ready", fmt.Sprintf("%s has %d/%d ready", deploy.Name, deploy.Status.ReadyReplicas, *deploy.Spec.Replicas), "high")
			}
			analysis.AddResource(fmt.Sprintf("deployments/%s/%s", ns, deploy.Name))
		}
	}
	analysis.AddNextCheck("Inspect karpenter controller logs")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: namespaces}}, nil
}

func (t *Toolset) handleCRStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	apiVersion := toString(args["apiVersion"])
	group := toString(args["group"])
	kind := toString(args["kind"])
	resource := toString(args["resource"])
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	selector := toString(args["labelSelector"])

	analysis := render.NewAnalysis()
	detected, _, groups, err := t.detectKarpenter(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "karpenter CRDs not detected")
		analysis.AddEvidence("groupsChecked", []string{"karpenter.sh"})
		analysis.AddNextCheck("Install Karpenter CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}
	if apiVersion == "" && kind == "" && resource == "" {
		err := errors.New("kind or resource required")
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, apiVersion, kind, resource, group)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}

	addObject := func(obj *unstructured.Unstructured) {
		if obj == nil {
			return
		}
		ref := t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName())
		analysis.AddResource(ref)
		status := evidence.StatusFromUnstructured(obj)
		if len(status) == 0 {
			analysis.AddEvidence(fmt.Sprintf("%s status", ref), "status field not found")
		} else {
			analysis.AddEvidence(fmt.Sprintf("%s status", ref), t.ctx.Redactor.RedactMap(status))
		}
		describe := render.DescribeAnalysis(ctx, t.ctx.Evidence, t.ctx.Redactor, gvr, obj)
		analysis.AddEvidence(fmt.Sprintf("%s describe", ref), t.ctx.Renderer.Render(describe))
	}

	var resources []string
	switch {
	case name != "":
		if namespaced {
			if namespace != "" {
				if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				obj, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						analysis.AddEvidence("status", "resource not found")
						analysis.AddNextCheck("Verify CR name and namespace")
						return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
					}
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				addObject(obj)
				resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, namespace, name))
			} else {
				namespaces, err := t.allowedNamespaces(ctx, req.User, namespace)
				if err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				var foundNamespaces []string
				var foundObj *unstructured.Unstructured
				for _, ns := range namespaces {
					if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
						return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
					}
					obj, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						continue
					}
					if err != nil {
						return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
					}
					foundNamespaces = append(foundNamespaces, ns)
					foundObj = obj
				}
				if len(foundNamespaces) == 0 {
					analysis.AddEvidence("status", "resource not found")
					analysis.AddNextCheck("Verify CR name or provide namespace")
					return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
				}
				if len(foundNamespaces) > 1 {
					err := fmt.Errorf("resource %q found in multiple namespaces: %s", name, strings.Join(foundNamespaces, ", "))
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				namespace = foundNamespaces[0]
				addObject(foundObj)
				resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, namespace, name))
			}
		} else {
			if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
				return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
			}
			obj, err := t.ctx.Clients.Dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					analysis.AddEvidence("status", "resource not found")
					analysis.AddNextCheck("Verify CR name")
					return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
				}
				return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
			}
			addObject(obj)
			resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, "", name))
		}
	default:
		if namespaced {
			if namespace != "" {
				if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
				if err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				for i := range list.Items {
					obj := &list.Items[i]
					addObject(obj)
					resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName()))
				}
			} else if req.User.Role == policy.RoleCluster {
				list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: selector})
				if err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				for i := range list.Items {
					obj := &list.Items[i]
					addObject(obj)
					resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName()))
				}
			} else {
				namespaces, err := t.allowedNamespaces(ctx, req.User, namespace)
				if err != nil {
					return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
				}
				for _, ns := range namespaces {
					list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
					if err != nil {
						return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
					}
					for i := range list.Items {
						obj := &list.Items[i]
						addObject(obj)
						resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName()))
					}
				}
			}
		} else {
			if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
				return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
			}
			list, err := t.ctx.Clients.Dynamic.Resource(gvr).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
			}
			for i := range list.Items {
				obj := &list.Items[i]
				addObject(obj)
				resources = append(resources, t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName()))
			}
		}
		if len(analysis.Evidence) == 0 {
			analysis.AddEvidence("status", "no matching resources found")
		}
	}

	analysis.AddNextCheck("Inspect Karpenter controller logs for CR reconciliation errors")
	return mcp.ToolResult{
		Data: t.ctx.Renderer.Render(analysis),
		Metadata: mcp.ToolMetadata{
			Namespaces: sliceIf(namespace),
			Resources:  resources,
		},
	}, nil
}

func (t *Toolset) handleNodeProvisioningDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	selector := toString(args["labelSelector"])
	podRefs := toStringSlice(args["pods"])
	analysis := render.NewAnalysis()

	namespaces, err := t.allowedNamespaces(ctx, req.User, namespace)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		pods, err := t.ctx.Clients.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for _, pod := range pods.Items {
			if len(podRefs) > 0 && !podInList(ns, pod.Name, podRefs) {
				continue
			}
			if pod.Status.Phase != corev1.PodPending {
				continue
			}
			reason, message := pendingReason(&pod)
			analysis.AddEvidence(fmt.Sprintf("%s/%s", ns, pod.Name), map[string]any{"reason": reason, "message": message})
			analysis.AddResource(fmt.Sprintf("pods/%s/%s", ns, pod.Name))
			if reason == "Unschedulable" {
				analysis.AddCause("Pending pod", message, "high")
			}
			obj, err := toUnstructured(&pod)
			if err == nil {
				events, err := t.ctx.Evidence.EventsForObject(ctx, obj)
				if err == nil && len(events) > 0 {
					analysis.AddEvidence(fmt.Sprintf("%s/%s events", ns, pod.Name), events)
				}
				describe := render.DescribeAnalysis(ctx, t.ctx.Evidence, t.ctx.Redactor, corev1.SchemeGroupVersion.WithResource("pods"), obj)
				analysis.AddEvidence(fmt.Sprintf("%s/%s describe", ns, pod.Name), t.ctx.Renderer.Render(describe))
			}
		}
	}
	if len(analysis.Evidence) == 0 {
		analysis.AddEvidence("status", "no pending pods found")
	}
	analysis.AddNextCheck("Review Karpenter provisioner constraints and node limits")
	if req.User.Role == policy.RoleCluster {
		nodes, err := t.ctx.Clients.Typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil {
			analysis.AddEvidence("nodeCount", len(nodes.Items))
		}
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) detectKarpenter(ctx context.Context) (bool, []string, []string, error) {
	present, groups, err := kube.GroupsPresent(t.ctx.Clients.Discovery, karpenterGroups)
	if err != nil {
		return false, nil, nil, err
	}
	namespaces, err := kube.ControlPlaneNamespaces(ctx, t.ctx.Clients, []string{karpenterSelector})
	if err != nil {
		return false, nil, nil, err
	}
	if len(namespaces) == 0 {
		if _, err := t.ctx.Clients.Typed.CoreV1().Namespaces().Get(ctx, karpenterNamespace, metav1.GetOptions{}); err == nil {
			namespaces = []string{karpenterNamespace}
		}
	}
	return present || len(namespaces) > 0, namespaces, groups, nil
}

func podInList(namespace, name string, list []string) bool {
	full := fmt.Sprintf("%s/%s", namespace, name)
	for _, item := range list {
		if item == name || item == full {
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

func toUnstructured(pod *corev1.Pod) (*unstructured.Unstructured, error) {
	if pod == nil {
		return nil, errors.New("pod is required")
	}
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: objMap}, nil
}

func (t *Toolset) allowedNamespaces(ctx context.Context, user policy.User, namespace string) ([]string, error) {
	if namespace != "" {
		return []string{namespace}, nil
	}
	if user.Role == policy.RoleCluster {
		list, err := t.ctx.Clients.Typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		var names []string
		for _, ns := range list.Items {
			names = append(names, ns.Name)
		}
		return names, nil
	}
	return append([]string{}, user.AllowedNamespaces...), nil
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

func toStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func sliceIf(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
