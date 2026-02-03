package linkerd

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

const linkerdNamespace = "linkerd"
const linkerdSelector = "linkerd.io/control-plane-component"

var linkerdGroups = []string{"linkerd.io", "policy.linkerd.io"}

func (t *Toolset) handleHealth(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, namespaces, groups, err := t.detectLinkerd(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "linkerd not detected")
		analysis.AddNextCheck("Confirm Linkerd is installed and namespace exists")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}
	if len(namespaces) == 0 {
		namespaces = []string{linkerdNamespace}
		analysis.AddEvidence("namespaceFallback", linkerdNamespace)
	}
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		deployments, err := t.ctx.Clients.Typed.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: linkerdSelector})
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for _, deploy := range deployments.Items {
			analysis.AddEvidence(deploy.Name, map[string]any{"ready": deploy.Status.ReadyReplicas, "desired": *deploy.Spec.Replicas})
			if deploy.Status.ReadyReplicas < *deploy.Spec.Replicas {
				analysis.AddCause("Control plane not ready", fmt.Sprintf("%s has %d/%d ready", deploy.Name, deploy.Status.ReadyReplicas, *deploy.Spec.Replicas), "high")
			}
			analysis.AddResource(fmt.Sprintf("deployments/%s/%s", ns, deploy.Name))
		}
	}
	analysis.AddNextCheck("Inspect linkerd control-plane pod logs")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: namespaces}}, nil
}

func (t *Toolset) handleProxyStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	selector := toString(req.Arguments["labelSelector"])
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	analysis := render.NewAnalysis()
	namespaces, err := t.allowedNamespaces(ctx, req.User, namespace)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	for _, ns := range namespaces {
		pods, err := t.ctx.Clients.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for _, pod := range pods.Items {
			if !hasLinkerdProxy(&pod) {
				continue
			}
			analysis.AddEvidence(fmt.Sprintf("%s/%s", ns, pod.Name), t.ctx.Evidence.PodStatusSummary(&pod))
			analysis.AddResource(fmt.Sprintf("pods/%s/%s", ns, pod.Name))
			if !isPodReady(&pod) {
				analysis.AddCause("Proxy not ready", fmt.Sprintf("%s/%s linkerd-proxy not ready", ns, pod.Name), "medium")
				if obj, err := toUnstructured(&pod); err == nil {
					describe := render.DescribeAnalysis(ctx, t.ctx.Evidence, t.ctx.Redactor, corev1.SchemeGroupVersion.WithResource("pods"), obj)
					analysis.AddEvidence(fmt.Sprintf("%s/%s describe", ns, pod.Name), t.ctx.Renderer.Render(describe))
				}
			}
		}
	}
	if len(analysis.Evidence) == 0 {
		analysis.AddEvidence("status", "no linkerd proxies found")
	}
	analysis.AddNextCheck("Check linkerd-proxy logs and injector configuration")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleIdentityIssues(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	detected, namespaces, _, err := t.detectLinkerd(ctx)
	analysis := render.NewAnalysis()
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "linkerd not detected")
		analysis.AddNextCheck("Install Linkerd or check namespace")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(namespaces) == 0 {
		namespaces = []string{linkerdNamespace}
		analysis.AddEvidence("namespaceFallback", linkerdNamespace)
	}
	var found bool
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		deployment, err := t.ctx.Clients.Typed.AppsV1().Deployments(ns).Get(ctx, "linkerd-identity", metav1.GetOptions{})
		if err != nil {
			continue
		}
		found = true
		analysis.AddEvidence("linkerd-identity", map[string]any{"ready": deployment.Status.ReadyReplicas, "desired": *deployment.Spec.Replicas})
		if deployment.Status.ReadyReplicas < *deployment.Spec.Replicas {
			analysis.AddCause("Identity service not ready", "linkerd-identity replicas unavailable", "high")
			analysis.AddNextCheck("Inspect linkerd-identity pod events and logs")
		}
		analysis.AddResource(fmt.Sprintf("deployments/%s/%s", ns, deployment.Name))
	}
	if !found {
		analysis.AddEvidence("status", "linkerd-identity deployment not found")
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: namespaces}}, nil
}

func (t *Toolset) handlePolicyDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	detected, _, _, err := t.detectLinkerd(ctx)
	analysis := render.NewAnalysis()
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "linkerd not detected")
		analysis.AddNextCheck("Install Linkerd control plane")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	groups, err := t.ctx.Clients.Discovery.ServerGroups()
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	var found bool
	for _, group := range groups.Groups {
		if group.Name == "policy.linkerd.io" {
			found = true
			break
		}
	}
	if !found {
		analysis.AddCause("Linkerd policy CRDs missing", "policy.linkerd.io API group not found", "medium")
		analysis.AddNextCheck("Check Linkerd policy controller installation")
	} else {
		analysis.AddEvidence("policyAPI", "policy.linkerd.io available")
	}
	analysis.AddNextCheck("Review Server and Authorization resources")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
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
	generalFetch := isGeneralMeshKind(kind, resource)
	detected, _, groups, err := t.detectLinkerd(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected && !generalFetch {
		analysis.AddEvidence("status", "linkerd CRDs not detected")
		analysis.AddEvidence("groupsChecked", []string{"linkerd.io", "policy.linkerd.io"})
		analysis.AddNextCheck("Install Linkerd CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	} else if !detected {
		analysis.AddEvidence("linkerdDetected", false)
	}
	if apiVersion == "" && kind == "" && resource == "" {
		err := errors.New("kind or resource required")
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if group == "" {
		group = inferGroupForKindResource(kind, resource)
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

	analysis.AddNextCheck("Inspect Linkerd controller logs for CR reconciliation errors")
	return mcp.ToolResult{
		Data: t.ctx.Renderer.Render(analysis),
		Metadata: mcp.ToolMetadata{
			Namespaces: sliceIf(namespace),
			Resources:  resources,
		},
	}, nil
}

func (t *Toolset) handleHTTPRouteStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "HTTPRoute")
}

func (t *Toolset) handleGatewayStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "Gateway")
}

func (t *Toolset) handleVirtualServiceStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "VirtualService")
}

func (t *Toolset) handleDestinationRuleStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "DestinationRule")
}

func (t *Toolset) handleKindStatus(ctx context.Context, req mcp.ToolRequest, kind string) (mcp.ToolResult, error) {
	args := map[string]any{
		"kind": kind,
	}
	copyArgIfPresent(args, req.Arguments, "name")
	copyArgIfPresent(args, req.Arguments, "namespace")
	copyArgIfPresent(args, req.Arguments, "labelSelector")
	copyArgIfPresent(args, req.Arguments, "apiVersion")
	return t.handleCRStatus(ctx, mcp.ToolRequest{
		Arguments: args,
		User:      req.User,
		Context:   req.Context,
	})
}

func (t *Toolset) detectLinkerd(ctx context.Context) (bool, []string, []string, error) {
	present, groups, err := kube.GroupsPresent(t.ctx.Clients.Discovery, linkerdGroups)
	if err != nil {
		return false, nil, nil, err
	}
	namespaces, err := kube.ControlPlaneNamespaces(ctx, t.ctx.Clients, []string{linkerdSelector})
	if err != nil {
		return false, nil, nil, err
	}
	if len(namespaces) == 0 {
		if _, err := t.ctx.Clients.Typed.CoreV1().Namespaces().Get(ctx, linkerdNamespace, metav1.GetOptions{}); err == nil {
			namespaces = []string{linkerdNamespace}
		}
	}
	return present || len(namespaces) > 0, namespaces, groups, nil
}

func inferGroupForKindResource(kind, resource string) string {
	kindKey := strings.ToLower(kind)
	resourceKey := strings.ToLower(resource)
	switch {
	case kindKey == "httproute" || resourceKey == "httproute" || resourceKey == "httproutes":
		return "gateway.networking.k8s.io"
	case kindKey == "gateway" || resourceKey == "gateway" || resourceKey == "gateways":
		return "gateway.networking.k8s.io"
	case kindKey == "virtualservice" || resourceKey == "virtualservice" || resourceKey == "virtualservices":
		return "networking.istio.io"
	case kindKey == "destinationrule" || resourceKey == "destinationrule" || resourceKey == "destinationrules":
		return "networking.istio.io"
	default:
		return ""
	}
}

func isGeneralMeshKind(kind, resource string) bool {
	if kind == "" && resource == "" {
		return false
	}
	return inferGroupForKindResource(kind, resource) != ""
}

func copyArgIfPresent(dst, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
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

func hasLinkerdProxy(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == "linkerd-proxy" {
			return true
		}
	}
	return false
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
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

func sliceIf(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func toUnstructured(pod *corev1.Pod) (*unstructured.Unstructured, error) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: objMap}, nil
}
