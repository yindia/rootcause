package k8s

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

type graphNode struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Group     string         `json:"group,omitempty"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

type graphEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
}

type graphBuilder struct {
	nodes map[string]graphNode
	edges []graphEdge
}

func newGraphBuilder() *graphBuilder {
	return &graphBuilder{nodes: map[string]graphNode{}}
}

type graphCache struct {
	servicesLoaded        bool
	endpointsLoaded       bool
	podsLoaded            bool
	deploymentsLoaded     bool
	replicasetsLoaded     bool
	statefulsetsLoaded    bool
	daemonsetsLoaded      bool
	ingressesLoaded       bool
	networkPoliciesLoaded bool
	namespacesLoaded      bool

	services    map[string]*corev1.Service
	serviceList []*corev1.Service

	endpoints    map[string]*corev1.Endpoints
	endpointList []*corev1.Endpoints

	pods    map[string]*corev1.Pod
	podList []*corev1.Pod

	deployments    map[string]*appsv1.Deployment
	deploymentList []*appsv1.Deployment

	replicasets    map[string]*appsv1.ReplicaSet
	replicasetList []*appsv1.ReplicaSet

	statefulsets    map[string]*appsv1.StatefulSet
	statefulsetList []*appsv1.StatefulSet

	daemonsets    map[string]*appsv1.DaemonSet
	daemonsetList []*appsv1.DaemonSet

	ingresses   map[string]*networkingv1.Ingress
	ingressList []*networkingv1.Ingress

	networkPolicies   map[string]*networkingv1.NetworkPolicy
	networkPolicyList []*networkingv1.NetworkPolicy

	namespaces    map[string]*corev1.Namespace
	namespaceList []*corev1.Namespace
}

func newGraphCache() *graphCache {
	return &graphCache{
		services:        map[string]*corev1.Service{},
		endpoints:       map[string]*corev1.Endpoints{},
		pods:            map[string]*corev1.Pod{},
		deployments:     map[string]*appsv1.Deployment{},
		replicasets:     map[string]*appsv1.ReplicaSet{},
		statefulsets:    map[string]*appsv1.StatefulSet{},
		daemonsets:      map[string]*appsv1.DaemonSet{},
		ingresses:       map[string]*networkingv1.Ingress{},
		networkPolicies: map[string]*networkingv1.NetworkPolicy{},
		namespaces:      map[string]*corev1.Namespace{},
	}
}

func (t *Toolset) buildGraphCache(ctx context.Context, namespace string, clusterAccess bool) (*graphCache, []string) {
	cache := newGraphCache()
	warnings := []string{}

	if list, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("service list failed: %v", err))
	} else {
		cache.servicesLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.services[item.Name] = item
			cache.serviceList = append(cache.serviceList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("endpoints list failed: %v", err))
	} else {
		cache.endpointsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.endpoints[item.Name] = item
			cache.endpointList = append(cache.endpointList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("pod list failed: %v", err))
	} else {
		cache.podsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.pods[item.Name] = item
			cache.podList = append(cache.podList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("deployment list failed: %v", err))
	} else {
		cache.deploymentsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.deployments[item.Name] = item
			cache.deploymentList = append(cache.deploymentList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("replicaset list failed: %v", err))
	} else {
		cache.replicasetsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.replicasets[item.Name] = item
			cache.replicasetList = append(cache.replicasetList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("statefulset list failed: %v", err))
	} else {
		cache.statefulsetsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.statefulsets[item.Name] = item
			cache.statefulsetList = append(cache.statefulsetList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("daemonset list failed: %v", err))
	} else {
		cache.daemonsetsLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.daemonsets[item.Name] = item
			cache.daemonsetList = append(cache.daemonsetList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("ingress list failed: %v", err))
	} else {
		cache.ingressesLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.ingresses[item.Name] = item
			cache.ingressList = append(cache.ingressList, item)
		}
	}

	if list, err := t.ctx.Clients.Typed.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		warnings = append(warnings, fmt.Sprintf("networkpolicy list failed: %v", err))
	} else {
		cache.networkPoliciesLoaded = true
		for i := range list.Items {
			item := &list.Items[i]
			cache.networkPolicies[item.Name] = item
			cache.networkPolicyList = append(cache.networkPolicyList, item)
		}
	}

	if clusterAccess {
		if list, err := t.ctx.Clients.Typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); err != nil {
			warnings = append(warnings, fmt.Sprintf("namespace list failed: %v", err))
		} else {
			cache.namespacesLoaded = true
			for i := range list.Items {
				item := &list.Items[i]
				cache.namespaces[item.Name] = item
				cache.namespaceList = append(cache.namespaceList, item)
			}
		}
	}

	return cache, warnings
}

func (g *graphBuilder) addNode(kind, group, namespace, name string, details map[string]any) string {
	id := nodeID(kind, group, namespace, name)
	if _, ok := g.nodes[id]; !ok {
		g.nodes[id] = graphNode{ID: id, Kind: kind, Group: group, Name: name, Namespace: namespace, Details: details}
	}
	return id
}

func (g *graphBuilder) addEdge(from, to, relation string) {
	g.edges = append(g.edges, graphEdge{From: from, To: to, Relation: relation})
}

func (g *graphBuilder) result() map[string]any {
	nodes := make([]graphNode, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return map[string]any{"nodes": nodes, "edges": g.edges}
}

func nodeID(kind, group, namespace, name string) string {
	kind = strings.ToLower(kind)
	group = strings.ToLower(group)
	if namespace == "" {
		if group != "" {
			return fmt.Sprintf("%s.%s/%s", kind, group, name)
		}
		return fmt.Sprintf("%s/%s", kind, name)
	}
	if group != "" {
		return fmt.Sprintf("%s.%s/%s/%s", kind, group, namespace, name)
	}
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

func (t *Toolset) handleGraph(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	kind := strings.ToLower(toString(args["kind"]))
	name := toString(args["name"])
	namespace := toString(args["namespace"])
	if kind == "" || name == "" || namespace == "" {
		return errorResult(errors.New("kind, name, and namespace are required")), errors.New("kind, name, and namespace are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}
	clusterAccess := req.User.Role == policy.RoleCluster
	if t.ctx.Cache != nil && t.ctx.Config != nil {
		ttlSeconds := t.ctx.Config.Cache.GraphTTLSeconds
		if ttlSeconds > 0 {
			key := graphCacheKey(kind, namespace, name, clusterAccess)
			if cached, ok := t.ctx.Cache.Get(key); ok {
				return mcp.ToolResult{Data: cached, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
			}
		}
	}

	graph := newGraphBuilder()
	cache, cacheWarnings := t.buildGraphCache(ctx, namespace, clusterAccess)
	warnings := append([]string{}, cacheWarnings...)

	switch kind {
	case "ingress":
		warn, err := t.addIngressGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "service":
		warn, err := t.addServiceGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "deployment":
		warn, err := t.addDeploymentGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "replicaset":
		warn, err := t.addReplicaSetGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "statefulset":
		warn, err := t.addStatefulSetGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "daemonset":
		warn, err := t.addDaemonSetGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	case "pod":
		warn, err := t.addPodGraph(ctx, graph, namespace, name, cache)
		if err != nil {
			return errorResult(err), err
		}
		warnings = append(warnings, warn...)
	default:
		return errorResult(errors.New("unsupported kind for graph")), errors.New("unsupported kind for graph")
	}

	warnings = append(warnings, t.addNetworkPolicyGraph(ctx, graph, namespace, cache)...)
	warnings = append(warnings, t.addMeshGraph(ctx, graph, namespace, cache)...)

	out := graph.result()
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	if t.ctx.Cache != nil && t.ctx.Config != nil {
		ttlSeconds := t.ctx.Config.Cache.GraphTTLSeconds
		if ttlSeconds > 0 {
			key := graphCacheKey(kind, namespace, name, clusterAccess)
			t.ctx.Cache.Set(key, out, time.Duration(ttlSeconds)*time.Second)
		}
	}
	return mcp.ToolResult{Data: out, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func graphCacheKey(kind, namespace, name string, clusterAccess bool) string {
	return fmt.Sprintf("graph:%s:%s:%s:%t", kind, namespace, name, clusterAccess)
}

var (
	istioGroups   = []string{"networking.istio.io", "security.istio.io"}
	linkerdGroups = []string{"linkerd.io", "policy.linkerd.io"}
	gatewayGroups = []string{"gateway.networking.k8s.io"}
)

func (t *Toolset) addMeshGraph(ctx context.Context, graph *graphBuilder, namespace string, cache *graphCache) []string {
	warnings := []string{}
	var services []corev1.Service
	if cache != nil && cache.servicesLoaded {
		for _, svc := range cache.serviceList {
			services = append(services, *svc)
		}
	} else {
		list, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return append(warnings, fmt.Sprintf("failed to list services for mesh graph: %v", err))
		}
		services = list.Items
	}
	serviceIndex := buildServiceIndex(namespace, services)

	if present, _, err := kube.GroupsPresent(t.ctx.Clients.Discovery, gatewayGroups); err != nil {
		warnings = append(warnings, fmt.Sprintf("gateway api discovery failed: %v", err))
	} else if present {
		warnings = append(warnings, t.addGroupResources(ctx, graph, namespace, "gateway.networking.k8s.io", serviceIndex, cache)...)
		warnings = append(warnings, t.addGatewayAPIGraph(ctx, graph, namespace, serviceIndex)...)
	}

	if present, _, err := kube.GroupsPresent(t.ctx.Clients.Discovery, istioGroups); err != nil {
		warnings = append(warnings, fmt.Sprintf("istio discovery failed: %v", err))
	} else if present {
		warnings = append(warnings, t.addGroupResources(ctx, graph, namespace, "networking.istio.io", serviceIndex, cache)...)
		warnings = append(warnings, t.addGroupResources(ctx, graph, namespace, "security.istio.io", serviceIndex, cache)...)
	}

	if present, _, err := kube.GroupsPresent(t.ctx.Clients.Discovery, linkerdGroups); err != nil {
		warnings = append(warnings, fmt.Sprintf("linkerd discovery failed: %v", err))
	} else if present {
		warnings = append(warnings, t.addGroupResources(ctx, graph, namespace, "linkerd.io", serviceIndex, cache)...)
		warnings = append(warnings, t.addGroupResources(ctx, graph, namespace, "policy.linkerd.io", serviceIndex, cache)...)
	}

	return warnings
}

func (t *Toolset) addNetworkPolicyGraph(ctx context.Context, graph *graphBuilder, namespace string, cache *graphCache) []string {
	warnings := []string{}
	var policies []networkingv1.NetworkPolicy
	if cache != nil && cache.networkPoliciesLoaded {
		for _, policy := range cache.networkPolicyList {
			policies = append(policies, *policy)
		}
	} else {
		list, err := t.ctx.Clients.Typed.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return append(warnings, fmt.Sprintf("networkpolicy list failed: %v", err))
		}
		policies = list.Items
	}
	for i := range policies {
		policy := &policies[i]
		policyID := graph.addNode("NetworkPolicy", "", namespace, policy.Name, nil)
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("networkpolicy %s selector invalid: %v", policy.Name, err))
			continue
		}
		pods, err := t.podsForSelector(ctx, namespace, selector, cache)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("networkpolicy %s pod lookup failed: %v", policy.Name, err))
			continue
		}
		for _, pod := range pods {
			podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
			graph.addEdge(policyID, podID, "selects")
		}
		if policyAppliesIngress(policy) && len(policy.Spec.Ingress) == 0 {
			for _, pod := range pods {
				podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
				graph.addEdge(podID, policyID, "blocked-by")
			}
		}
		if policyAppliesEgress(policy) && len(policy.Spec.Egress) == 0 {
			for _, pod := range pods {
				podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
				graph.addEdge(podID, policyID, "egress-blocked-by")
			}
		}
		for _, rule := range policy.Spec.Ingress {
			for _, peer := range rule.From {
				t.addNetworkPolicyPeerEdges(ctx, graph, namespace, policyID, peer, "allows-from", cache, &warnings)
			}
		}
		for _, rule := range policy.Spec.Egress {
			for _, peer := range rule.To {
				t.addNetworkPolicyPeerEdges(ctx, graph, namespace, policyID, peer, "allows-to", cache, &warnings)
			}
		}
	}
	return warnings
}

func buildServiceIndex(namespace string, services []corev1.Service) map[string]string {
	index := map[string]string{}
	for _, svc := range services {
		name := svc.Name
		short := name
		index[short] = name
		index[fmt.Sprintf("%s.%s", name, namespace)] = name
		index[fmt.Sprintf("%s.%s.svc", name, namespace)] = name
		index[fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)] = name
	}
	return index
}

type groupResource struct {
	GVR        schema.GroupVersionResource
	Kind       string
	Namespaced bool
	Group      string
	Version    string
	Resource   string
	ShortNames []string
}

func (t *Toolset) addGroupResources(ctx context.Context, graph *graphBuilder, namespace, group string, serviceIndex map[string]string, cache *graphCache) []string {
	warnings := []string{}
	resources, err := t.groupResources(group)
	if err != nil {
		return append(warnings, fmt.Sprintf("resource discovery failed for %s: %v", group, err))
	}
	for _, res := range resources {
		if res.Namespaced {
			list, err := t.ctx.Clients.Dynamic.Resource(res.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				warnings = append(warnings, fmt.Sprintf("%s list failed: %v", res.GVR.Resource, err))
				continue
			}
			for i := range list.Items {
				obj := &list.Items[i]
				nodeID := graph.addNode(res.Kind, res.Group, namespace, obj.GetName(), map[string]any{
					"apiVersion": obj.GetAPIVersion(),
					"resource":   res.Resource,
					"scope":      "namespaced",
				})
				_ = nodeID
				warnings = append(warnings, t.addMeshEdges(ctx, graph, obj, res, namespace, serviceIndex, cache)...)
			}
			continue
		}
		list, err := t.ctx.Clients.Dynamic.Resource(res.GVR).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			warnings = append(warnings, fmt.Sprintf("%s list failed: %v", res.GVR.Resource, err))
			continue
		}
		for i := range list.Items {
			obj := &list.Items[i]
			nodeID := graph.addNode(res.Kind, res.Group, "", obj.GetName(), map[string]any{
				"apiVersion": obj.GetAPIVersion(),
				"resource":   res.Resource,
				"scope":      "cluster",
			})
			_ = nodeID
		}
	}
	return warnings
}

func (t *Toolset) groupResources(group string) ([]groupResource, error) {
	lists, err := t.ctx.Clients.Discovery.ServerPreferredResources()
	if err != nil && !discoveryErrorIsPartial(err) {
		return nil, err
	}
	var resources []groupResource
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil || gv.Group != group {
			continue
		}
		for _, res := range list.APIResources {
			if res.Name == "" || strings.Contains(res.Name, "/") {
				continue
			}
			resources = append(resources, groupResource{
				GVR:        schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: res.Name},
				Kind:       res.Kind,
				Namespaced: res.Namespaced,
				Group:      gv.Group,
				Version:    gv.Version,
				Resource:   res.Name,
				ShortNames: res.ShortNames,
			})
		}
	}
	return resources, nil
}

func discoveryErrorIsPartial(err error) bool {
	if err == nil {
		return false
	}
	return discovery.IsGroupDiscoveryFailedError(err)
}

func (t *Toolset) addMeshEdges(ctx context.Context, graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string, serviceIndex map[string]string, cache *graphCache) []string {
	warnings := []string{}
	kind := strings.ToLower(res.Kind)

	warnings = append(warnings, t.linkByHosts(graph, obj, res, namespace, serviceIndex)...)
	warnings = append(warnings, t.linkBySelector(ctx, graph, obj, res, namespace, cache)...)
	warnings = append(warnings, t.linkByTargetRef(graph, obj, res, namespace)...)
	warnings = append(warnings, t.linkByServerRef(graph, obj, res, namespace)...)

	if kind == "virtualservice" {
		for _, gateway := range nestedStringSlice(obj, "spec", "gateways") {
			if gateway == "mesh" {
				continue
			}
			name := gateway
			if strings.Contains(gateway, "/") {
				parts := strings.SplitN(gateway, "/", 2)
				name = parts[1]
			}
			gwID := graph.addNode("Gateway", "networking.istio.io", namespace, name, nil)
			graph.addEdge(nodeID(res.Kind, res.Group, namespace, obj.GetName()), gwID, "attached-to")
		}
	}

	if kind == "serviceprofile" && res.Group == "linkerd.io" {
		if svcName, ok := serviceIndex[obj.GetName()]; ok {
			svcID := graph.addNode("Service", "", namespace, svcName, nil)
			graph.addEdge(nodeID(res.Kind, res.Group, namespace, obj.GetName()), svcID, "profiles")
		}
	}
	if kind == "authorizationpolicy" && res.Group == "security.istio.io" {
		warnings = append(warnings, t.linkIstioAuthorizationPolicyToServiceAccounts(graph, obj, res, namespace)...)
	}

	return warnings
}

func (t *Toolset) linkByHosts(graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string, serviceIndex map[string]string) []string {
	warnings := []string{}
	hosts := []string{}
	if host := nestedString(obj, "spec", "host"); host != "" {
		hosts = append(hosts, host)
	}
	if extra := nestedStringSlice(obj, "spec", "hosts"); len(extra) > 0 {
		hosts = append(hosts, extra...)
	}
	if len(hosts) == 0 {
		return warnings
	}
	sourceID := nodeID(res.Kind, res.Group, namespace, obj.GetName())
	for _, host := range hosts {
		if svcName, ok := serviceIndex[host]; ok {
			svcID := graph.addNode("Service", "", namespace, svcName, nil)
			graph.addEdge(sourceID, svcID, "routes-to")
		}
	}
	return warnings
}

func (t *Toolset) linkBySelector(ctx context.Context, graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string, cache *graphCache) []string {
	warnings := []string{}
	selectors := []map[string]string{
		nestedStringMap(obj, "spec", "selector", "matchLabels"),
		nestedStringMap(obj, "spec", "workloadSelector", "labels"),
		nestedStringMap(obj, "spec", "podSelector", "matchLabels"),
	}
	sourceID := nodeID(res.Kind, res.Group, namespace, obj.GetName())
	for _, selector := range selectors {
		if len(selector) == 0 {
			continue
		}
		pods, err := t.podsForSelector(ctx, namespace, labels.SelectorFromSet(selector), cache)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("selector lookup failed for %s: %v", res.Resource, err))
			continue
		}
		for _, pod := range pods {
			podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
			graph.addEdge(sourceID, podID, "applies-to")
		}
	}
	return warnings
}

func (t *Toolset) linkByTargetRef(graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string) []string {
	warnings := []string{}
	ref, _, _ := unstructured.NestedMap(obj.Object, "spec", "targetRef")
	if len(ref) == 0 {
		return warnings
	}
	kind := toString(ref["kind"])
	name := toString(ref["name"])
	group := toString(ref["group"])
	if group == "" {
		group = toString(ref["apiGroup"])
	}
	if kind == "" || name == "" {
		return warnings
	}
	sourceID := nodeID(res.Kind, res.Group, namespace, obj.GetName())
	targetID := graph.addNode(kind, group, namespace, name, nil)
	graph.addEdge(sourceID, targetID, "targets")
	return warnings
}

func (t *Toolset) linkByServerRef(graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string) []string {
	warnings := []string{}
	server := nestedString(obj, "spec", "server")
	if server == "" {
		server = nestedString(obj, "spec", "server", "name")
	}
	if server == "" {
		return warnings
	}
	sourceID := nodeID(res.Kind, res.Group, namespace, obj.GetName())
	serverID := graph.addNode("Server", "policy.linkerd.io", namespace, server, nil)
	graph.addEdge(sourceID, serverID, "authorizes")
	return warnings
}

func policyAppliesIngress(policy *networkingv1.NetworkPolicy) bool {
	if policy == nil {
		return false
	}
	if len(policy.Spec.PolicyTypes) == 0 {
		return true
	}
	for _, policyType := range policy.Spec.PolicyTypes {
		if policyType == networkingv1.PolicyTypeIngress {
			return true
		}
	}
	return false
}

func policyAppliesEgress(policy *networkingv1.NetworkPolicy) bool {
	if policy == nil {
		return false
	}
	for _, policyType := range policy.Spec.PolicyTypes {
		if policyType == networkingv1.PolicyTypeEgress {
			return true
		}
	}
	return false
}

func (t *Toolset) addNetworkPolicyPeerEdges(ctx context.Context, graph *graphBuilder, namespace, policyID string, peer networkingv1.NetworkPolicyPeer, relation string, cache *graphCache, warnings *[]string) {
	if peer.IPBlock != nil {
		ipDetails := map[string]any{}
		if len(peer.IPBlock.Except) > 0 {
			ipDetails["except"] = peer.IPBlock.Except
		}
		ipID := graph.addNode("IPBlock", "", "", peer.IPBlock.CIDR, ipDetails)
		graph.addEdge(policyID, ipID, relation)
	}

	targetNamespaces := []string{}
	if peer.NamespaceSelector != nil {
		nsSelector, err := metav1.LabelSelectorAsSelector(peer.NamespaceSelector)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("networkpolicy namespace selector invalid: %v", err))
		} else if cache != nil && cache.namespacesLoaded {
			for _, ns := range cache.namespaceList {
				if nsSelector.Matches(labels.Set(ns.Labels)) {
					targetNamespaces = append(targetNamespaces, ns.Name)
				}
			}
		} else {
			*warnings = append(*warnings, "namespace selector present but namespaces not available")
		}
	}
	if len(targetNamespaces) == 0 {
		targetNamespaces = []string{namespace}
	}

	if peer.PodSelector == nil {
		if peer.NamespaceSelector != nil {
			for _, ns := range targetNamespaces {
				nsID := graph.addNode("Namespace", "", "", ns, nil)
				graph.addEdge(policyID, nsID, relation+"-namespace")
			}
		}
		return
	}
	podSelector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
	if err != nil {
		*warnings = append(*warnings, fmt.Sprintf("networkpolicy pod selector invalid: %v", err))
		return
	}
	for _, ns := range targetNamespaces {
		if ns != namespace {
			nsID := graph.addNode("Namespace", "", "", ns, nil)
			graph.addEdge(policyID, nsID, relation+"-namespace")
			continue
		}
		pods, err := t.podsForSelector(ctx, ns, podSelector, cache)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("networkpolicy peer pod lookup failed: %v", err))
			continue
		}
		for _, pod := range pods {
			podID := graph.addNode("Pod", "", ns, pod.Name, map[string]any{"phase": pod.Status.Phase})
			graph.addEdge(policyID, podID, relation)
		}
	}
}

func (t *Toolset) linkIstioAuthorizationPolicyToServiceAccounts(graph *graphBuilder, obj *unstructured.Unstructured, res groupResource, namespace string) []string {
	warnings := []string{}
	sourceID := nodeID(res.Kind, res.Group, namespace, obj.GetName())
	for _, principal := range istioAuthorizationPolicyPrincipals(obj) {
		ns, sa, ok := parseIstioServiceAccountPrincipal(principal)
		if !ok {
			continue
		}
		if ns == "" {
			ns = namespace
		}
		saID := graph.addNode("ServiceAccount", "", ns, sa, nil)
		graph.addEdge(sourceID, saID, "authorizes")
	}
	return warnings
}

func istioAuthorizationPolicyPrincipals(obj *unstructured.Unstructured) []string {
	out := []string{}
	rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		froms, ok := ruleMap["from"].([]any)
		if !ok {
			continue
		}
		for _, from := range froms {
			fromMap, ok := from.(map[string]any)
			if !ok {
				continue
			}
			source, ok := fromMap["source"].(map[string]any)
			if !ok {
				continue
			}
			if principals, ok := source["principals"].([]any); ok {
				for _, principal := range principals {
					if value, ok := principal.(string); ok && value != "" {
						out = append(out, value)
					}
				}
			}
		}
	}
	return out
}

func parseIstioServiceAccountPrincipal(principal string) (string, string, bool) {
	value := strings.TrimSpace(principal)
	if value == "" {
		return "", "", false
	}
	if strings.HasPrefix(value, "spiffe://") {
		value = strings.TrimPrefix(value, "spiffe://")
	}
	if idx := strings.Index(value, "cluster.local/ns/"); idx >= 0 {
		value = value[idx+len("cluster.local/ns/"):]
	} else if strings.HasPrefix(value, "ns/") {
		value = strings.TrimPrefix(value, "ns/")
	}
	parts := strings.Split(value, "/sa/")
	if len(parts) != 2 {
		return "", "", false
	}
	ns := strings.TrimSpace(parts[0])
	sa := strings.TrimSpace(parts[1])
	if ns == "" || sa == "" {
		return "", "", false
	}
	return ns, sa, true
}

func nestedStringMap(obj *unstructured.Unstructured, fields ...string) map[string]string {
	if obj == nil {
		return nil
	}
	value, _, _ := unstructured.NestedStringMap(obj.Object, fields...)
	return value
}

func (t *Toolset) addGatewayAPIGraph(ctx context.Context, graph *graphBuilder, namespace string, serviceIndex map[string]string) []string {
	warnings := []string{}
	gvrGateway, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "Gateway", "", "gateway.networking.k8s.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("gateway api resolve failed: %v", err))
	}
	gvrHTTPRoute, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "HTTPRoute", "", "gateway.networking.k8s.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("httproute resolve failed: %v", err))
	}
	gateways, err := t.ctx.Clients.Dynamic.Resource(gvrGateway).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("gateway list failed: %v", err))
	}
	routes, err := t.ctx.Clients.Dynamic.Resource(gvrHTTPRoute).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("httproute list failed: %v", err))
	}
	for i := range gateways.Items {
		gw := &gateways.Items[i]
		graph.addNode("Gateway", "gateway.networking.k8s.io", namespace, gw.GetName(), nil)
	}
	for i := range routes.Items {
		route := &routes.Items[i]
		routeID := graph.addNode("HTTPRoute", "gateway.networking.k8s.io", namespace, route.GetName(), nil)
		for _, parent := range nestedParentRefs(route) {
			if parent != "" {
				gwID := graph.addNode("Gateway", "gateway.networking.k8s.io", namespace, parent, nil)
				graph.addEdge(routeID, gwID, "attached-to")
			}
		}
		for _, backend := range nestedBackendRefs(route) {
			if svcName, ok := serviceIndex[backend]; ok {
				svcID := graph.addNode("Service", "", namespace, svcName, nil)
				graph.addEdge(routeID, svcID, "routes-to")
			}
		}
	}
	return warnings
}

func (t *Toolset) addIstioGraph(ctx context.Context, graph *graphBuilder, namespace string, serviceIndex map[string]string) []string {
	warnings := []string{}
	virtualGVR, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "VirtualService", "", "networking.istio.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("virtualservice resolve failed: %v", err))
	}
	destGVR, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "DestinationRule", "", "networking.istio.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("destinationrule resolve failed: %v", err))
	}
	gatewayGVR, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "Gateway", "", "networking.istio.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("istio gateway resolve failed: %v", err))
	}
	virtuals, err := t.ctx.Clients.Dynamic.Resource(virtualGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("virtualservice list failed: %v", err))
	}
	dests, err := t.ctx.Clients.Dynamic.Resource(destGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("destinationrule list failed: %v", err))
	}
	gateways, err := t.ctx.Clients.Dynamic.Resource(gatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("istio gateway list failed: %v", err))
	}
	for i := range gateways.Items {
		gw := &gateways.Items[i]
		graph.addNode("Gateway", "networking.istio.io", namespace, gw.GetName(), nil)
	}
	for i := range virtuals.Items {
		vs := &virtuals.Items[i]
		vsID := graph.addNode("VirtualService", "networking.istio.io", namespace, vs.GetName(), nil)
		hosts := nestedStringSlice(vs, "spec", "hosts")
		for _, host := range hosts {
			if svcName, ok := serviceIndex[host]; ok {
				svcID := graph.addNode("Service", "", namespace, svcName, nil)
				graph.addEdge(vsID, svcID, "routes-to")
			}
		}
		for _, gateway := range nestedStringSlice(vs, "spec", "gateways") {
			if gateway == "mesh" {
				continue
			}
			name := gateway
			if strings.Contains(gateway, "/") {
				parts := strings.SplitN(gateway, "/", 2)
				name = parts[1]
			}
			gwID := graph.addNode("Gateway", "networking.istio.io", namespace, name, nil)
			graph.addEdge(vsID, gwID, "attached-to")
		}
	}
	for i := range dests.Items {
		dr := &dests.Items[i]
		drID := graph.addNode("DestinationRule", "networking.istio.io", namespace, dr.GetName(), nil)
		host := nestedString(dr, "spec", "host")
		if host == "" {
			continue
		}
		if svcName, ok := serviceIndex[host]; ok {
			svcID := graph.addNode("Service", "", namespace, svcName, nil)
			graph.addEdge(drID, svcID, "policy-for")
		}
	}
	return warnings
}

func (t *Toolset) addLinkerdGraph(ctx context.Context, graph *graphBuilder, namespace string, serviceIndex map[string]string) []string {
	warnings := []string{}
	spGVR, _, err := kube.ResolveResourceBestEffort(t.ctx.Clients.Mapper, t.ctx.Clients.Discovery, "", "ServiceProfile", "", "linkerd.io")
	if err != nil {
		return append(warnings, fmt.Sprintf("serviceprofile resolve failed: %v", err))
	}
	profiles, err := t.ctx.Clients.Dynamic.Resource(spGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		warnings = append(warnings, fmt.Sprintf("serviceprofile list failed: %v", err))
	}
	for i := range profiles.Items {
		profile := &profiles.Items[i]
		profileID := graph.addNode("ServiceProfile", "linkerd.io", namespace, profile.GetName(), nil)
		if svcName, ok := serviceIndex[profile.GetName()]; ok {
			svcID := graph.addNode("Service", "", namespace, svcName, nil)
			graph.addEdge(profileID, svcID, "profiles")
		}
	}
	return warnings
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

func nestedParentRefs(obj *unstructured.Unstructured) []string {
	refs := []string{}
	items, _, _ := unstructured.NestedSlice(obj.Object, "spec", "parentRefs")
	for _, raw := range items {
		ref, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if kind, ok := ref["kind"].(string); ok && kind != "" && kind != "Gateway" {
			continue
		}
		if name, ok := ref["name"].(string); ok && name != "" {
			refs = append(refs, name)
		}
	}
	return refs
}

func nestedBackendRefs(obj *unstructured.Unstructured) []string {
	out := []string{}
	rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "rules")
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		backendRefs, _, _ := unstructured.NestedSlice(ruleMap, "backendRefs")
		for _, backend := range backendRefs {
			bm, ok := backend.(map[string]any)
			if !ok {
				continue
			}
			if kind, ok := bm["kind"].(string); ok && kind != "" && kind != "Service" {
				continue
			}
			if name, ok := bm["name"].(string); ok && name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

func (t *Toolset) addIngressGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	ingress, err := t.getIngress(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	ingressID := graph.addNode("Ingress", "", namespace, ingress.Name, nil)
	services := ingressBackendServices(ingress)
	if len(services) == 0 {
		warnings = append(warnings, "ingress has no backend services")
	}
	for _, svc := range services {
		svcID := graph.addNode("Service", "", namespace, svc, nil)
		graph.addEdge(ingressID, svcID, "routes-to")
		warn, err := t.addServiceGraph(ctx, graph, namespace, svc, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("service not found: %s", svc))
				continue
			}
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}
	return warnings, nil
}

func (t *Toolset) addServiceGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	service, err := t.getService(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	serviceID := graph.addNode("Service", "", namespace, service.Name, nil)
	endpoints, err := t.getEndpoints(ctx, cache, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			warnings = append(warnings, "endpoints not found for service")
		} else {
			return warnings, err
		}
	} else {
		endpointsID := graph.addNode("Endpoints", "", namespace, endpoints.Name, nil)
		graph.addEdge(serviceID, endpointsID, "selects")
		pods := podsFromEndpoints(endpoints)
		for _, podName := range pods {
			podID := graph.addNode("Pod", "", namespace, podName, nil)
			graph.addEdge(endpointsID, podID, "targets")
			warn, err := t.addPodGraph(ctx, graph, namespace, podName, cache)
			if err != nil {
				if apierrors.IsNotFound(err) {
					warnings = append(warnings, fmt.Sprintf("pod not found: %s", podName))
					continue
				}
				return warnings, err
			}
			warnings = append(warnings, warn...)
		}
	}

	if len(service.Spec.Selector) == 0 {
		warnings = append(warnings, "service has no selector")
		return warnings, nil
	}
	selector := labels.Set(service.Spec.Selector).AsSelector()
	pods, err := t.podsForSelector(ctx, namespace, selector, cache)
	if err != nil {
		return warnings, err
	}
	for _, pod := range pods {
		podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
		graph.addEdge(serviceID, podID, "selects")
		warn, err := t.addPodGraph(ctx, graph, namespace, pod.Name, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("pod not found: %s", pod.Name))
				continue
			}
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}
	return warnings, nil
}

func (t *Toolset) addDeploymentGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	deployment, err := t.getDeployment(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	depID := graph.addNode("Deployment", "", namespace, deployment.Name, map[string]any{"ready": deployment.Status.ReadyReplicas, "desired": derefInt32(deployment.Spec.Replicas)})
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return warnings, err
	}
	replicasets, err := t.replicasetsForSelector(ctx, namespace, selector, cache)
	if err != nil {
		return warnings, err
	}
	for _, rs := range replicasets {
		if !ownedBy(&rs.ObjectMeta, "Deployment", deployment.Name) {
			continue
		}
		rsID := graph.addNode("ReplicaSet", "", namespace, rs.Name, map[string]any{"ready": rs.Status.ReadyReplicas, "desired": derefInt32(rs.Spec.Replicas)})
		graph.addEdge(depID, rsID, "owns")
		warn, err := t.addReplicaSetPods(ctx, graph, namespace, &rs, cache)
		if err != nil {
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}

	warnings = append(warnings, t.linkServicesForLabels(ctx, graph, namespace, deployment.Spec.Template.Labels, "Deployment", deployment.Name, cache)...)
	return warnings, nil
}

func (t *Toolset) addReplicaSetGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	rs, err := t.getReplicaSet(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	rsID := graph.addNode("ReplicaSet", "", namespace, rs.Name, map[string]any{"ready": rs.Status.ReadyReplicas, "desired": derefInt32(rs.Spec.Replicas)})
	warnings := []string{}
	if owner := firstOwner(&rs.ObjectMeta); owner != nil {
		if owner.Kind == "Deployment" {
			depID := graph.addNode("Deployment", "", namespace, owner.Name, nil)
			graph.addEdge(rsID, depID, "owned-by")
		}
	}
	warn, err := t.addReplicaSetPods(ctx, graph, namespace, rs, cache)
	if err != nil {
		return warnings, err
	}
	warnings = append(warnings, warn...)
	warnings = append(warnings, t.linkServicesForLabels(ctx, graph, namespace, rs.Spec.Template.Labels, "ReplicaSet", rs.Name, cache)...)
	return warnings, nil
}

func (t *Toolset) addReplicaSetPods(ctx context.Context, graph *graphBuilder, namespace string, rs *appsv1.ReplicaSet, cache *graphCache) ([]string, error) {
	warnings := []string{}
	if rs == nil {
		return warnings, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	if err != nil {
		return warnings, err
	}
	pods, err := t.podsForSelector(ctx, namespace, selector, cache)
	if err != nil {
		return warnings, err
	}
	rsID := graph.addNode("ReplicaSet", "", namespace, rs.Name, nil)
	for _, pod := range pods {
		podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
		graph.addEdge(rsID, podID, "owns")
		warn, err := t.addPodGraph(ctx, graph, namespace, pod.Name, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("pod not found: %s", pod.Name))
				continue
			}
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}
	return warnings, nil
}

func (t *Toolset) addStatefulSetGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	ss, err := t.getStatefulSet(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	ssID := graph.addNode("StatefulSet", "", namespace, ss.Name, map[string]any{"ready": ss.Status.ReadyReplicas, "desired": derefInt32(ss.Spec.Replicas)})
	selector, err := metav1.LabelSelectorAsSelector(ss.Spec.Selector)
	if err != nil {
		return warnings, err
	}
	pods, err := t.podsForSelector(ctx, namespace, selector, cache)
	if err != nil {
		return warnings, err
	}
	for _, pod := range pods {
		podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
		graph.addEdge(ssID, podID, "owns")
		warn, err := t.addPodGraph(ctx, graph, namespace, pod.Name, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("pod not found: %s", pod.Name))
				continue
			}
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}
	if ss.Spec.ServiceName != "" {
		serviceID := graph.addNode("Service", "", namespace, ss.Spec.ServiceName, nil)
		graph.addEdge(ssID, serviceID, "headless-service")
		warn, err := t.addServiceGraph(ctx, graph, namespace, ss.Spec.ServiceName, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("service not found: %s", ss.Spec.ServiceName))
			} else {
				return warnings, err
			}
		} else {
			warnings = append(warnings, warn...)
		}
	}
	return warnings, nil
}

func (t *Toolset) addDaemonSetGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	ds, err := t.getDaemonSet(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	dsID := graph.addNode("DaemonSet", "", namespace, ds.Name, map[string]any{"ready": ds.Status.NumberReady, "desired": ds.Status.DesiredNumberScheduled})
	selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return warnings, err
	}
	pods, err := t.podsForSelector(ctx, namespace, selector, cache)
	if err != nil {
		return warnings, err
	}
	for _, pod := range pods {
		podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
		graph.addEdge(dsID, podID, "owns")
		warn, err := t.addPodGraph(ctx, graph, namespace, pod.Name, cache)
		if err != nil {
			if apierrors.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("pod not found: %s", pod.Name))
				continue
			}
			return warnings, err
		}
		warnings = append(warnings, warn...)
	}
	warnings = append(warnings, t.linkServicesForLabels(ctx, graph, namespace, ds.Spec.Template.Labels, "DaemonSet", ds.Name, cache)...)
	return warnings, nil
}

func (t *Toolset) addPodGraph(ctx context.Context, graph *graphBuilder, namespace, name string, cache *graphCache) ([]string, error) {
	warnings := []string{}
	pod, err := t.getPod(ctx, cache, namespace, name)
	if err != nil {
		return nil, err
	}
	podID := graph.addNode("Pod", "", namespace, pod.Name, map[string]any{"phase": pod.Status.Phase})
	owner := firstOwner(&pod.ObjectMeta)
	if owner == nil {
		warnings = append(warnings, "pod has no owner references")
		return warnings, nil
	}
	switch owner.Kind {
	case "ReplicaSet":
		rsID := graph.addNode("ReplicaSet", "", namespace, owner.Name, nil)
		graph.addEdge(podID, rsID, "owned-by")
		rs, err := t.getReplicaSet(ctx, cache, namespace, owner.Name)
		if err == nil {
			if depOwner := firstOwner(&rs.ObjectMeta); depOwner != nil && depOwner.Kind == "Deployment" {
				depID := graph.addNode("Deployment", "", namespace, depOwner.Name, nil)
				graph.addEdge(rsID, depID, "owned-by")
			}
		}
	case "StatefulSet":
		ssID := graph.addNode("StatefulSet", "", namespace, owner.Name, nil)
		graph.addEdge(podID, ssID, "owned-by")
	case "DaemonSet":
		dsID := graph.addNode("DaemonSet", "", namespace, owner.Name, nil)
		graph.addEdge(podID, dsID, "owned-by")
	case "Deployment":
		depID := graph.addNode("Deployment", "", namespace, owner.Name, nil)
		graph.addEdge(podID, depID, "owned-by")
	default:
		graph.addNode(owner.Kind, "", namespace, owner.Name, nil)
		graph.addEdge(podID, nodeID(owner.Kind, "", namespace, owner.Name), "owned-by")
	}
	warnings = append(warnings, t.linkServicesForLabels(ctx, graph, namespace, pod.Labels, "Pod", pod.Name, cache)...)
	return warnings, nil
}

func (t *Toolset) linkServicesForLabels(ctx context.Context, graph *graphBuilder, namespace string, labelsMap map[string]string, targetKind, targetName string, cache *graphCache) []string {
	warnings := []string{}
	if len(labelsMap) == 0 {
		return warnings
	}
	var services []corev1.Service
	if cache != nil && cache.servicesLoaded {
		for _, svc := range cache.serviceList {
			services = append(services, *svc)
		}
	} else {
		list, err := t.ctx.Clients.Typed.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return append(warnings, fmt.Sprintf("failed to list services: %v", err))
		}
		services = list.Items
	}
	for _, svc := range services {
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		selector := labels.Set(svc.Spec.Selector).AsSelector()
		if selector.Matches(labels.Set(labelsMap)) {
			svcID := graph.addNode("Service", "", namespace, svc.Name, nil)
			targetID := graph.addNode(targetKind, "", namespace, targetName, nil)
			graph.addEdge(svcID, targetID, "selects")
		}
	}
	return warnings
}

func (t *Toolset) getService(ctx context.Context, cache *graphCache, namespace, name string) (*corev1.Service, error) {
	if cache != nil && cache.servicesLoaded {
		if svc, ok := cache.services[name]; ok {
			return svc, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "services"}, name)
	}
	return t.ctx.Clients.Typed.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getEndpoints(ctx context.Context, cache *graphCache, namespace, name string) (*corev1.Endpoints, error) {
	if cache != nil && cache.endpointsLoaded {
		if eps, ok := cache.endpoints[name]; ok {
			return eps, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "endpoints"}, name)
	}
	return t.ctx.Clients.Typed.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getPod(ctx context.Context, cache *graphCache, namespace, name string) (*corev1.Pod, error) {
	if cache != nil && cache.podsLoaded {
		if pod, ok := cache.pods[name]; ok {
			return pod, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, name)
	}
	return t.ctx.Clients.Typed.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getDeployment(ctx context.Context, cache *graphCache, namespace, name string) (*appsv1.Deployment, error) {
	if cache != nil && cache.deploymentsLoaded {
		if dep, ok := cache.deployments[name]; ok {
			return dep, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "deployments"}, name)
	}
	return t.ctx.Clients.Typed.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getReplicaSet(ctx context.Context, cache *graphCache, namespace, name string) (*appsv1.ReplicaSet, error) {
	if cache != nil && cache.replicasetsLoaded {
		if rs, ok := cache.replicasets[name]; ok {
			return rs, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "replicasets"}, name)
	}
	return t.ctx.Clients.Typed.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getStatefulSet(ctx context.Context, cache *graphCache, namespace, name string) (*appsv1.StatefulSet, error) {
	if cache != nil && cache.statefulsetsLoaded {
		if ss, ok := cache.statefulsets[name]; ok {
			return ss, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "statefulsets"}, name)
	}
	return t.ctx.Clients.Typed.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getDaemonSet(ctx context.Context, cache *graphCache, namespace, name string) (*appsv1.DaemonSet, error) {
	if cache != nil && cache.daemonsetsLoaded {
		if ds, ok := cache.daemonsets[name]; ok {
			return ds, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "daemonsets"}, name)
	}
	return t.ctx.Clients.Typed.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) getIngress(ctx context.Context, cache *graphCache, namespace, name string) (*networkingv1.Ingress, error) {
	if cache != nil && cache.ingressesLoaded {
		if ing, ok := cache.ingresses[name]; ok {
			return ing, nil
		}
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "ingresses"}, name)
	}
	return t.ctx.Clients.Typed.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (t *Toolset) podsForSelector(ctx context.Context, namespace string, selector labels.Selector, cache *graphCache) ([]corev1.Pod, error) {
	if cache != nil && cache.podsLoaded {
		out := make([]corev1.Pod, 0, len(cache.podList))
		for _, pod := range cache.podList {
			if selector.Matches(labels.Set(pod.Labels)) {
				out = append(out, *pod)
			}
		}
		return out, nil
	}
	return t.ctx.Evidence.RelatedPods(ctx, namespace, selector)
}

func (t *Toolset) replicasetsForSelector(ctx context.Context, namespace string, selector labels.Selector, cache *graphCache) ([]appsv1.ReplicaSet, error) {
	if cache != nil && cache.replicasetsLoaded {
		out := make([]appsv1.ReplicaSet, 0, len(cache.replicasetList))
		for _, rs := range cache.replicasetList {
			if selector.Matches(labels.Set(rs.Labels)) {
				out = append(out, *rs)
			}
		}
		return out, nil
	}
	list, err := t.ctx.Clients.Typed.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func ingressBackendServices(ingress *networkingv1.Ingress) []string {
	if ingress == nil {
		return nil
	}
	found := map[string]struct{}{}
	if ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
		found[ingress.Spec.DefaultBackend.Service.Name] = struct{}{}
	}
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				found[path.Backend.Service.Name] = struct{}{}
			}
		}
	}
	var out []string
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func podsFromEndpoints(endpoints *corev1.Endpoints) []string {
	if endpoints == nil {
		return nil
	}
	found := map[string]struct{}{}
	for _, subset := range endpoints.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
				found[addr.TargetRef.Name] = struct{}{}
			}
		}
	}
	var out []string
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func firstOwner(meta *metav1.ObjectMeta) *metav1.OwnerReference {
	if meta == nil || len(meta.OwnerReferences) == 0 {
		return nil
	}
	owner := meta.OwnerReferences[0]
	return &owner
}

func ownedBy(meta *metav1.ObjectMeta, kind, name string) bool {
	if meta == nil {
		return false
	}
	for _, owner := range meta.OwnerReferences {
		if owner.Kind == kind && owner.Name == name {
			return true
		}
	}
	return false
}

func derefInt32(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}
