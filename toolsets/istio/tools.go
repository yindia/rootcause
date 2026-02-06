package istio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

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

type namespaceInjectionSummary struct {
	Namespace        string  `json:"namespace"`
	TotalPods        int     `json:"totalPods"`
	ProxyPods        int     `json:"proxyPods"`
	InjectionPercent float64 `json:"injectionPercent"`
}

type hostRecord struct {
	Host    string   `json:"host"`
	Sources []string `json:"sources,omitempty"`
}

type externalHostRecord struct {
	Host         string `json:"host"`
	SourceKind   string `json:"sourceKind"`
	Namespace    string `json:"namespace,omitempty"`
	ObjectName   string `json:"objectName,omitempty"`
	ServiceEntry bool   `json:"serviceEntry"`
}

func (t *Toolset) handleConfigSummary(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
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
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	kinds := []struct {
		Kind  string
		Group string
	}{
		{Kind: "VirtualService", Group: "networking.istio.io"},
		{Kind: "DestinationRule", Group: "networking.istio.io"},
		{Kind: "Gateway", Group: "networking.istio.io"},
		{Kind: "ServiceEntry", Group: "networking.istio.io"},
		{Kind: "Sidecar", Group: "networking.istio.io"},
		{Kind: "EnvoyFilter", Group: "networking.istio.io"},
		{Kind: "PeerAuthentication", Group: "security.istio.io"},
		{Kind: "AuthorizationPolicy", Group: "security.istio.io"},
		{Kind: "RequestAuthentication", Group: "security.istio.io"},
		{Kind: "Telemetry", Group: "telemetry.istio.io"},
	}
	counts := map[string]int{}
	for _, item := range kinds {
		gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", item.Kind, "", item.Group)
		if err != nil {
			continue
		}
		items, _, err := t.listObjects(ctx, req.User, gvr, namespaced, namespace, "")
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		counts[item.Kind] = len(items)
	}
	if len(counts) == 0 {
		analysis.AddEvidence("status", "no istio configuration resources found")
	} else {
		analysis.AddEvidence("counts", counts)
	}
	analysis.AddNextCheck("Review istiod logs for reconciliation errors")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleServiceMeshHosts(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
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
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	hostSources := map[string][]string{}
	addHost := func(host, source string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		hostSources[host] = append(hostSources[host], source)
	}
	addHostsFor := func(kind, group string, extract func(*unstructured.Unstructured) []string) error {
		gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", kind, "", group)
		if err != nil {
			return nil
		}
		items, _, err := t.listObjects(ctx, req.User, gvr, namespaced, namespace, "")
		if err != nil {
			return err
		}
		for i := range items {
			obj := &items[i]
			ref := t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName())
			analysis.AddResource(ref)
			source := fmt.Sprintf("%s %s/%s", kind, obj.GetNamespace(), obj.GetName())
			for _, host := range extract(obj) {
				addHost(host, source)
			}
		}
		return nil
	}
	if err := addHostsFor("VirtualService", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		return nestedStringSlice(obj, "spec", "hosts")
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := addHostsFor("DestinationRule", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		host := nestedString(obj, "spec", "host")
		if host == "" {
			return nil
		}
		return []string{host}
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := addHostsFor("Gateway", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		return gatewayServerHosts(obj)
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := addHostsFor("ServiceEntry", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		return nestedStringSlice(obj, "spec", "hosts")
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	var hosts []hostRecord
	for host, sources := range hostSources {
		sort.Strings(sources)
		hosts = append(hosts, hostRecord{Host: host, Sources: sources})
	}
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Host < hosts[j].Host
	})
	if len(hosts) == 0 {
		analysis.AddEvidence("status", "no mesh hosts found")
	} else {
		analysis.AddEvidence("hosts", t.ctx.Redactor.RedactValue(hosts))
		analysis.AddEvidence("totalHosts", len(hosts))
	}
	analysis.AddNextCheck("Review VirtualService and DestinationRule host mappings")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleDiscoverNamespaces(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	selector := toString(req.Arguments["labelSelector"])
	analysis := render.NewAnalysis()
	detected, _, err := t.detectIstio(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "istio not detected")
		analysis.AddNextCheck("Install Istio or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	namespaces, err := t.allowedNamespaces(ctx, req.User, namespace)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	var summaries []namespaceInjectionSummary
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(req.User, ns, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		pods, err := t.ctx.Clients.Typed.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		total := len(pods.Items)
		proxyCount := 0
		for i := range pods.Items {
			if hasIstioProxy(&pods.Items[i]) {
				proxyCount++
			}
		}
		percent := 0.0
		if total > 0 {
			percent = (float64(proxyCount) / float64(total)) * 100
		}
		summaries = append(summaries, namespaceInjectionSummary{
			Namespace:        ns,
			TotalPods:        total,
			ProxyPods:        proxyCount,
			InjectionPercent: percent,
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].InjectionPercent > summaries[j].InjectionPercent
	})
	if len(summaries) == 0 {
		analysis.AddEvidence("status", "no namespaces found")
	} else {
		analysis.AddEvidence("namespaces", summaries)
	}
	analysis.AddNextCheck("Check namespace labels and injection policies")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handlePodsByService(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	service := toString(req.Arguments["service"])
	if namespace == "" || service == "" {
		err := errors.New("namespace and service required")
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	analysis := render.NewAnalysis()
	svc, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			analysis.AddEvidence("status", "service not found")
			analysis.AddNextCheck("Verify service name and namespace")
			return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
		}
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	analysis.AddResource(fmt.Sprintf("services/%s/%s", namespace, service))
	analysis.AddEvidence("service", map[string]any{
		"selector": svc.Spec.Selector,
		"ports":    svc.Spec.Ports,
		"type":     svc.Spec.Type,
	})
	if len(svc.Spec.Selector) == 0 {
		analysis.AddEvidence("status", "service has no selector")
		analysis.AddNextCheck("Check Endpoints or EndpointSlice for manual backends")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	selector := labels.SelectorFromSet(svc.Spec.Selector)
	pods, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	var podSummaries []map[string]any
	for i := range pods.Items {
		pod := &pods.Items[i]
		summary := t.ctx.Evidence.PodStatusSummary(pod)
		summary["name"] = pod.Name
		summary["node"] = pod.Spec.NodeName
		summary["istioProxy"] = hasIstioProxy(pod)
		podSummaries = append(podSummaries, summary)
		analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, pod.Name))
	}
	analysis.AddEvidence("pods", podSummaries)
	endpoints, err := t.ctx.Evidence.EndpointsForService(ctx, namespace, service)
	if err == nil && endpoints != nil {
		analysis.AddEvidence("endpoints", endpoints.Subsets)
	}
	if len(podSummaries) == 0 {
		analysis.AddEvidence("status", "no pods matched service selector")
		analysis.AddNextCheck("Verify service selector labels")
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) handleExternalDependencyCheck(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
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
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
	}
	services, _, err := t.listServices(ctx, req.User, namespace)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	internalHosts := buildServiceHostSet(services)
	serviceEntryHosts, err := t.collectServiceEntryHosts(ctx, req, namespace)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}

	var externalHosts []externalHostRecord
	missingHosts := map[string]struct{}{}
	checkHosts := func(kind, group string, extract func(*unstructured.Unstructured) []string) error {
		gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", kind, "", group)
		if err != nil {
			return nil
		}
		items, _, err := t.listObjects(ctx, req.User, gvr, namespaced, namespace, "")
		if err != nil {
			return err
		}
		for i := range items {
			obj := &items[i]
			ref := t.ctx.Evidence.ResourceRef(gvr, obj.GetNamespace(), obj.GetName())
			analysis.AddResource(ref)
			for _, host := range extract(obj) {
				host = strings.TrimSpace(host)
				if host == "" || host == "mesh" {
					continue
				}
				if _, ok := internalHosts[host]; ok {
					continue
				}
				hasEntry := matchServiceEntry(host, serviceEntryHosts)
				externalHosts = append(externalHosts, externalHostRecord{
					Host:         host,
					SourceKind:   kind,
					Namespace:    obj.GetNamespace(),
					ObjectName:   obj.GetName(),
					ServiceEntry: hasEntry,
				})
				if !hasEntry {
					missingHosts[host] = struct{}{}
				}
			}
		}
		return nil
	}
	if err := checkHosts("VirtualService", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		return nestedStringSlice(obj, "spec", "hosts")
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := checkHosts("DestinationRule", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		host := nestedString(obj, "spec", "host")
		if host == "" {
			return nil
		}
		return []string{host}
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := checkHosts("Gateway", "networking.istio.io", func(obj *unstructured.Unstructured) []string {
		return gatewayServerHosts(obj)
	}); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if len(externalHosts) == 0 {
		analysis.AddEvidence("status", "no external hosts found")
	} else {
		analysis.AddEvidence("externalHosts", t.ctx.Redactor.RedactValue(externalHosts))
	}
	if len(missingHosts) > 0 {
		missing := make([]string, 0, len(missingHosts))
		for host := range missingHosts {
			missing = append(missing, host)
		}
		sort.Strings(missing)
		analysis.AddCause("External host missing ServiceEntry", fmt.Sprintf("%d host(s) missing ServiceEntry: %s", len(missing), strings.Join(missing, ", ")), "medium")
		analysis.AddNextCheck("Define ServiceEntry resources for external hosts")
	} else {
		analysis.AddNextCheck("Verify external egress policies and DNS resolution")
	}
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleProxyClusters(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "clusters")
}

func (t *Toolset) handleProxyListeners(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "listeners")
}

func (t *Toolset) handleProxyRoutes(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "routes")
}

func (t *Toolset) handleProxyEndpoints(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "endpoints")
}

func (t *Toolset) handleProxyBootstrap(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "bootstrap")
}

func (t *Toolset) handleProxyConfigDump(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.handleProxyAdmin(ctx, req, "config_dump")
}

func (t *Toolset) handleProxyAdmin(ctx context.Context, req mcp.ToolRequest, path string) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	podName := toString(req.Arguments["pod"])
	if namespace == "" || podName == "" {
		err := errors.New("namespace and pod required")
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	analysis := render.NewAnalysis()
	pod, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			analysis.AddEvidence("status", "pod not found")
			analysis.AddNextCheck("Verify pod name and namespace")
			return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
		}
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !hasIstioProxy(pod) {
		analysis.AddEvidence("status", "pod does not have istio-proxy")
		analysis.AddNextCheck("Choose a pod with an injected istio-proxy sidecar")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	adminPort := toInt(req.Arguments["adminPort"], 15000)
	format := toString(req.Arguments["format"])
	if format == "" && path != "config_dump" {
		format = "json"
	}
	raw, err := t.proxyAdminRequest(ctx, namespace, podName, adminPort, path, format)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	analysis.AddResource(fmt.Sprintf("pods/%s/%s", namespace, podName))
	analysis.AddEvidence("proxyData", t.ctx.Redactor.RedactValue(parseProxyPayload(raw)))
	analysis.AddNextCheck("Compare proxy config with expected routes and clusters")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
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

func (t *Toolset) listObjects(ctx context.Context, user policy.User, gvr schema.GroupVersionResource, namespaced bool, namespace, selector string) ([]unstructured.Unstructured, []string, error) {
	opts := metav1.ListOptions{LabelSelector: selector}
	if namespaced {
		if namespace != "" {
			if err := t.ctx.Policy.CheckNamespace(user, namespace, true); err != nil {
				return nil, nil, err
			}
			list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(namespace).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			return list.Items, []string{namespace}, nil
		}
		if user.Role == policy.RoleCluster {
			list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(metav1.NamespaceAll).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			return list.Items, nil, nil
		}
		var items []unstructured.Unstructured
		namespaces := append([]string{}, user.AllowedNamespaces...)
		for _, ns := range namespaces {
			if err := t.ctx.Policy.CheckNamespace(user, ns, true); err != nil {
				return nil, nil, err
			}
			list, err := t.ctx.Clients.Dynamic.Resource(gvr).Namespace(ns).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, list.Items...)
		}
		return items, namespaces, nil
	}
	if err := t.ctx.Policy.CheckNamespace(user, "", false); err != nil {
		return nil, nil, err
	}
	list, err := t.ctx.Clients.Dynamic.Resource(gvr).List(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	return list.Items, nil, nil
}

func (t *Toolset) listServices(ctx context.Context, user policy.User, namespace string) ([]corev1.Service, []string, error) {
	if namespace != "" {
		if err := t.ctx.Policy.CheckNamespace(user, namespace, true); err != nil {
			return nil, nil, err
		}
		list, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		return list.Items, []string{namespace}, nil
	}
	if user.Role == policy.RoleCluster {
		list, err := t.ctx.Clients.Typed.CoreV1().Services(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		return list.Items, nil, nil
	}
	var services []corev1.Service
	namespaces := append([]string{}, user.AllowedNamespaces...)
	for _, ns := range namespaces {
		if err := t.ctx.Policy.CheckNamespace(user, ns, true); err != nil {
			return nil, nil, err
		}
		list, err := t.ctx.Clients.Typed.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		services = append(services, list.Items...)
	}
	return services, namespaces, nil
}

func buildServiceHostSet(services []corev1.Service) map[string]struct{} {
	hosts := map[string]struct{}{}
	for _, svc := range services {
		name := svc.Name
		namespace := svc.Namespace
		addHost := func(host string) {
			if host == "" {
				return
			}
			hosts[host] = struct{}{}
		}
		addHost(name)
		addHost(fmt.Sprintf("%s.%s", name, namespace))
		addHost(fmt.Sprintf("%s.%s.svc", name, namespace))
		addHost(fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace))
	}
	return hosts
}

func (t *Toolset) collectServiceEntryHosts(ctx context.Context, req mcp.ToolRequest, namespace string) ([]string, error) {
	gvr, namespaced, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "ServiceEntry", "", "networking.istio.io")
	if err != nil {
		return nil, nil
	}
	items, _, err := t.listObjects(ctx, req.User, gvr, namespaced, namespace, "")
	if err != nil {
		return nil, err
	}
	var hosts []string
	for i := range items {
		hosts = append(hosts, nestedStringSlice(&items[i], "spec", "hosts")...)
	}
	return hosts, nil
}

func matchServiceEntry(host string, patterns []string) bool {
	for _, pattern := range patterns {
		if hostMatchesPattern(host, pattern) {
			return true
		}
	}
	return false
}

func hostMatchesPattern(host, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(host, strings.TrimPrefix(pattern, "*"))
	}
	if strings.HasSuffix(pattern, ".*") {
		return strings.HasPrefix(host, strings.TrimSuffix(pattern, ".*"))
	}
	return host == pattern
}

func gatewayServerHosts(obj *unstructured.Unstructured) []string {
	if obj == nil {
		return nil
	}
	servers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "servers")
	var hosts []string
	for _, server := range servers {
		serverMap, ok := server.(map[string]any)
		if !ok {
			continue
		}
		serverHosts, _, _ := unstructured.NestedStringSlice(serverMap, "hosts")
		hosts = append(hosts, serverHosts...)
	}
	return hosts
}

func (t *Toolset) proxyAdminRequest(ctx context.Context, namespace, pod string, port int, path, format string) ([]byte, error) {
	params := map[string]string{}
	if format != "" && path != "config_dump" {
		params["format"] = format
	}
	return t.ctx.Clients.Typed.CoreV1().Pods(namespace).ProxyGet("http", pod, strconv.Itoa(port), path, params).DoRaw(ctx)
}

func parseProxyPayload(raw []byte) any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var decoded any
		if err := json.Unmarshal(trimmed, &decoded); err == nil {
			return decoded
		}
	}
	return string(trimmed)
}

func nestedString(obj *unstructured.Unstructured, fields ...string) string {
	if obj == nil {
		return ""
	}
	value, _, _ := unstructured.NestedString(obj.Object, fields...)
	return value
}

func nestedStringSlice(obj *unstructured.Unstructured, fields ...string) []string {
	if obj == nil {
		return nil
	}
	value, _, _ := unstructured.NestedStringSlice(obj.Object, fields...)
	return value
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

func toInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
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
