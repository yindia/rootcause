package istio

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

const istioNamespace = "istio-system"

var istioGroups = []string{"networking.istio.io", "security.istio.io"}
var istioSelectors = []string{"app=istiod", "istio=pilot", "app.kubernetes.io/name=istiod", "app.kubernetes.io/part-of=istio"}

func (t *Toolset) handleHealth(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, groups, err := t.detectIstio(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "istio not detected")
		analysis.AddEvidence("groupsChecked", istioGroups)
		analysis.AddNextCheck("Install Istio or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}
	namespaces, err := kube.ControlPlaneNamespaces(ctx, t.ctx.Clients, istioSelectors)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if len(namespaces) == 0 {
		namespaces = []string{istioNamespace}
		analysis.AddEvidence("namespaceFallback", istioNamespace)
	}
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		deployments := map[string]struct{}{}
		deployList := []string{}
		for _, selector := range istioSelectors {
			list, err := t.ctx.Clients.Typed.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil {
				return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
			}
			for _, deploy := range list.Items {
				key := fmt.Sprintf("%s/%s", ns, deploy.Name)
				if _, exists := deployments[key]; exists {
					continue
				}
				deployments[key] = struct{}{}
				deployList = append(deployList, deploy.Name)
				analysis.AddEvidence(deploy.Name, map[string]any{"ready": deploy.Status.ReadyReplicas, "desired": *deploy.Spec.Replicas})
				if deploy.Status.ReadyReplicas < *deploy.Spec.Replicas {
					analysis.AddCause("Control plane not ready", fmt.Sprintf("%s has %d/%d ready", deploy.Name, deploy.Status.ReadyReplicas, *deploy.Spec.Replicas), "high")
				}
				analysis.AddResource(fmt.Sprintf("deployments/%s/%s", ns, deploy.Name))
			}
		}
		if len(deployList) == 0 {
			analysis.AddEvidence(fmt.Sprintf("%s status", ns), "no istio control-plane deployments found")
		}
	}
	analysis.AddNextCheck("Inspect istiod and ingress gateway logs")
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
			if !hasIstioProxy(&pod) {
				continue
			}
			analysis.AddEvidence(fmt.Sprintf("%s/%s", ns, pod.Name), t.ctx.Evidence.PodStatusSummary(&pod))
			analysis.AddResource(fmt.Sprintf("pods/%s/%s", ns, pod.Name))
			if !isPodReady(&pod) {
				analysis.AddCause("Proxy not ready", fmt.Sprintf("%s/%s istio-proxy not ready", ns, pod.Name), "medium")
				if obj, err := toUnstructured(&pod); err == nil {
					describe := render.DescribeAnalysis(ctx, t.ctx.Evidence, t.ctx.Redactor, corev1.SchemeGroupVersion.WithResource("pods"), obj)
					analysis.AddEvidence(fmt.Sprintf("%s/%s describe", ns, pod.Name), t.ctx.Renderer.Render(describe))
				}
			}
		}
	}
	if len(analysis.Evidence) == 0 {
		analysis.AddEvidence("status", "no istio proxies found")
	}
	analysis.AddNextCheck("Check istio-proxy logs and injector configuration")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
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
	generalFetch := isGatewayAPIKind(kind, resource)
	detected, groups, err := t.detectIstio(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected && !generalFetch {
		analysis.AddEvidence("status", "istio CRDs not detected")
		analysis.AddEvidence("groupsChecked", istioGroups)
		analysis.AddNextCheck("Install Istio CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	} else if !detected {
		analysis.AddEvidence("istioDetected", false)
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

	analysis.AddNextCheck("Inspect Istio controller logs for CR reconciliation errors")
	return mcp.ToolResult{
		Data: t.ctx.Renderer.Render(analysis),
		Metadata: mcp.ToolMetadata{
			Namespaces: sliceIf(namespace),
			Resources:  resources,
		},
	}, nil
}

func (t *Toolset) handleVirtualServiceStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "VirtualService")
}

func (t *Toolset) handleDestinationRuleStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "DestinationRule")
}

func (t *Toolset) handleGatewayStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "Gateway")
}

func (t *Toolset) handleHTTPRouteStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleKindStatus(ctx, req, "HTTPRoute")
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

func (t *Toolset) detectIstio(ctx context.Context) (bool, []string, error) {
	present, groups, err := kube.GroupsPresent(t.ctx.Clients.Discovery, istioGroups)
	if err != nil {
		return false, nil, err
	}
	namespaces, err := kube.ControlPlaneNamespaces(ctx, t.ctx.Clients, istioSelectors)
	if err != nil {
		return false, nil, err
	}
	if len(namespaces) == 0 {
		if _, err := t.ctx.Clients.Typed.CoreV1().Namespaces().Get(ctx, istioNamespace, metav1.GetOptions{}); err == nil {
			namespaces = []string{istioNamespace}
		}
	}
	return present || len(namespaces) > 0, groups, nil
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

func isGatewayAPIKind(kind, resource string) bool {
	if kind == "" && resource == "" {
		return false
	}
	group := inferGroupForKindResource(kind, resource)
	return group == "gateway.networking.k8s.io"
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

func hasIstioProxy(pod *corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == "istio-proxy" {
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

func copyArgIfPresent(dst, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}
