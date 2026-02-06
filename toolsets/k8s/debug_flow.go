package k8s

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"rootcause/internal/mcp"
)

type flowNode struct {
	ID        string
	Kind      string
	Name      string
	Namespace string
}

type flowStep struct {
	Step   int               `json:"step"`
	Node   map[string]any    `json:"node"`
	Tool   string            `json:"tool"`
	Args   map[string]any    `json:"args"`
	Result any               `json:"result,omitempty"`
	Error  string            `json:"error,omitempty"`
	Notes  map[string]string `json:"notes,omitempty"`
}

type graphView struct {
	nodes map[string]flowNode
	edges []graphEdge
}

func (t *Toolset) handleDebugFlow(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	namespace := toString(req.Arguments["namespace"])
	kind := strings.ToLower(toString(req.Arguments["kind"]))
	name := toString(req.Arguments["name"])
	scenario := strings.ToLower(toString(req.Arguments["scenario"]))
	maxSteps := toInt(req.Arguments["maxSteps"], 20)
	if maxSteps <= 0 {
		maxSteps = 20
	}
	if namespace == "" || kind == "" || name == "" || scenario == "" {
		return errorResult(errors.New("namespace, kind, name, and scenario are required")), errors.New("namespace, kind, name, and scenario are required")
	}
	if err := t.ctx.Policy.CheckNamespace(req.User, namespace, true); err != nil {
		return errorResult(err), err
	}

	graphResult, err := t.ctx.CallTool(ctx, req.User, "k8s.graph", map[string]any{
		"kind":      kind,
		"name":      name,
		"namespace": namespace,
	})
	if err != nil {
		return errorResult(err), err
	}
	graph, warnings, err := parseGraph(graphResult.Data)
	if err != nil {
		return errorResult(err), err
	}

	entryID := nodeID(kind, "", namespace, name)
	if _, ok := graph.nodes[entryID]; !ok {
		if matched := findNodeByName(graph.nodes, kind, namespace, name); matched != "" {
			entryID = matched
		}
	}

	steps := []flowStep{}
	stepCount := 0
	addStep := func(node flowNode, tool string, args map[string]any, notes map[string]string) {
		if stepCount >= maxSteps {
			return
		}
		stepCount++
		step := flowStep{
			Step:  stepCount,
			Node:  map[string]any{"kind": node.Kind, "name": node.Name, "namespace": node.Namespace, "id": node.ID},
			Tool:  tool,
			Args:  args,
			Notes: notes,
		}
		if tool != "" {
			result, err := t.ctx.CallTool(ctx, req.User, tool, args)
			if err != nil {
				step.Error = err.Error()
			} else {
				step.Result = result.Data
			}
		}
		steps = append(steps, step)
	}

	switch scenario {
	case "traffic":
		t.buildTrafficFlow(graph, entryID, namespace, addStep)
	case "pending":
		t.buildPendingFlow(graph, entryID, namespace, addStep)
	case "crashloop":
		t.buildCrashloopFlow(graph, entryID, namespace, addStep)
	case "autoscaling":
		t.buildAutoscalingFlow(graph, entryID, namespace, addStep)
	case "networkpolicy":
		t.buildNetworkPolicyFlow(graph, entryID, namespace, addStep)
	case "mesh":
		t.buildMeshFlow(graph, entryID, namespace, addStep)
	default:
		return errorResult(fmt.Errorf("unsupported scenario: %s", scenario)), fmt.Errorf("unsupported scenario: %s", scenario)
	}

	data := map[string]any{
		"entry": map[string]any{
			"kind":      kind,
			"name":      name,
			"namespace": namespace,
		},
		"scenario": scenario,
		"graph":    graphResult.Data,
		"steps":    steps,
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}
	return mcp.ToolResult{Data: data, Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}}}, nil
}

func (t *Toolset) buildTrafficFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	entry := graph.nodes[entryID]
	addStep(entry, "k8s.describe", map[string]any{"kind": entry.Kind, "name": entry.Name, "namespace": namespace}, map[string]string{"reason": "entry"})

	services := relatedByKind(graph, []string{entryID}, "Service")
	if len(services) == 0 && strings.EqualFold(entry.Kind, "Service") {
		services = append(services, entry)
	}
	for _, svc := range services {
		addStep(svc, "k8s.network_debug", map[string]any{"namespace": namespace, "service": svc.Name}, map[string]string{"reason": "service path"})
	}

	pods := relatedPodsForServices(graph, services)
	for _, pod := range pods {
		addStep(pod, "k8s.describe", map[string]any{"kind": "Pod", "name": pod.Name, "namespace": namespace}, map[string]string{"reason": "backend pod"})
	}

	workloads := relatedWorkloadsForPods(graph, pods)
	for _, wl := range workloads {
		addStep(wl, "k8s.describe", map[string]any{"kind": wl.Kind, "name": wl.Name, "namespace": namespace}, map[string]string{"reason": "owner workload"})
	}
}

func (t *Toolset) buildPendingFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	addStep(flowNode{Kind: "Namespace", Name: namespace, Namespace: namespace, ID: namespace}, "k8s.scheduling_debug", map[string]any{"namespace": namespace}, map[string]string{"reason": "pending pods"})
	pods := podsFromEntry(graph, entryID)
	for _, pod := range pods {
		addStep(pod, "k8s.describe", map[string]any{"kind": "Pod", "name": pod.Name, "namespace": namespace}, map[string]string{"reason": "pending pod"})
		addStep(pod, "k8s.storage_debug", map[string]any{"namespace": namespace, "pod": pod.Name}, map[string]string{"reason": "pvc checks"})
	}
}

func (t *Toolset) buildCrashloopFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	addStep(flowNode{Kind: "Namespace", Name: namespace, Namespace: namespace, ID: namespace}, "k8s.crashloop_debug", map[string]any{"namespace": namespace}, map[string]string{"reason": "crashloop pods"})
	pods := podsFromEntry(graph, entryID)
	for _, pod := range pods {
		addStep(pod, "k8s.describe", map[string]any{"kind": "Pod", "name": pod.Name, "namespace": namespace}, map[string]string{"reason": "crashloop pod"})
		addStep(pod, "k8s.config_debug", map[string]any{"namespace": namespace, "pod": pod.Name}, map[string]string{"reason": "config refs"})
		addStep(pod, "k8s.storage_debug", map[string]any{"namespace": namespace, "pod": pod.Name}, map[string]string{"reason": "volume checks"})
	}
}

func (t *Toolset) buildAutoscalingFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	entry := graph.nodes[entryID]
	addStep(entry, "k8s.describe", map[string]any{"kind": entry.Kind, "name": entry.Name, "namespace": namespace}, map[string]string{"reason": "workload"})
	addStep(flowNode{Kind: "Namespace", Name: namespace, Namespace: namespace, ID: namespace}, "k8s.hpa_debug", map[string]any{"namespace": namespace}, map[string]string{"reason": "HPA"})
	addStep(flowNode{Kind: "Namespace", Name: namespace, Namespace: namespace, ID: namespace}, "k8s.vpa_debug", map[string]any{"namespace": namespace}, map[string]string{"reason": "VPA"})
	addStep(flowNode{Kind: "Namespace", Name: namespace, Namespace: namespace, ID: namespace}, "k8s.resource_usage", map[string]any{"namespace": namespace}, map[string]string{"reason": "metrics-server"})
}

func (t *Toolset) buildNetworkPolicyFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	entry := graph.nodes[entryID]
	addStep(entry, "k8s.describe", map[string]any{"kind": entry.Kind, "name": entry.Name, "namespace": namespace}, map[string]string{"reason": "entry"})

	services := relatedByKind(graph, []string{entryID}, "Service")
	if len(services) == 0 && strings.EqualFold(entry.Kind, "Service") {
		services = append(services, entry)
	}
	for _, svc := range services {
		addStep(svc, "k8s.network_debug", map[string]any{"namespace": namespace, "service": svc.Name}, map[string]string{"reason": "network policy check"})
	}

	pods := relatedPodsForServices(graph, services)
	policies := policiesForPods(graph, pods)
	for _, policy := range policies {
		addStep(policy, "k8s.describe", map[string]any{"kind": "NetworkPolicy", "name": policy.Name, "namespace": namespace}, map[string]string{"reason": "policy details"})
	}
}

func (t *Toolset) buildMeshFlow(graph graphView, entryID, namespace string, addStep func(flowNode, string, map[string]any, map[string]string)) {
	entry := graph.nodes[entryID]
	addStep(entry, "k8s.describe", map[string]any{"kind": entry.Kind, "name": entry.Name, "namespace": namespace}, map[string]string{"reason": "entry"})
	if hasKindGroup(graph, "VirtualService", "networking.istio.io") || hasKindGroup(graph, "DestinationRule", "networking.istio.io") {
		addStep(flowNode{Kind: "Istio", Name: "mesh", Namespace: namespace, ID: "istio"}, "istio.service_mesh_hosts", map[string]any{"namespace": namespace}, map[string]string{"reason": "istio mesh"})
	}
	if hasKindGroup(graph, "ServiceProfile", "linkerd.io") {
		addStep(flowNode{Kind: "Linkerd", Name: "mesh", Namespace: namespace, ID: "linkerd"}, "linkerd.policy_debug", map[string]any{}, map[string]string{"reason": "linkerd mesh"})
	}
}

func parseGraph(data any) (graphView, []string, error) {
	view := graphView{nodes: map[string]flowNode{}}
	out := map[string]any{}
	switch v := data.(type) {
	case map[string]any:
		out = v
	default:
		return view, nil, errors.New("invalid graph payload")
	}
	if rawNodes, ok := out["nodes"].([]any); ok {
		for _, raw := range rawNodes {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			node := flowNode{
				ID:        toString(item["id"]),
				Kind:      toString(item["kind"]),
				Name:      toString(item["name"]),
				Namespace: toString(item["namespace"]),
			}
			if node.ID == "" {
				continue
			}
			view.nodes[node.ID] = node
		}
	}
	if rawEdges, ok := out["edges"].([]any); ok {
		for _, raw := range rawEdges {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			view.edges = append(view.edges, graphEdge{
				From:     toString(item["from"]),
				To:       toString(item["to"]),
				Relation: toString(item["relation"]),
			})
		}
	}
	var warnings []string
	if rawWarnings, ok := out["warnings"].([]any); ok {
		for _, item := range rawWarnings {
			if s, ok := item.(string); ok {
				warnings = append(warnings, s)
			}
		}
	}
	return view, warnings, nil
}

func findNodeByName(nodes map[string]flowNode, kind, namespace, name string) string {
	for id, node := range nodes {
		if strings.EqualFold(node.Kind, kind) && node.Name == name && node.Namespace == namespace {
			return id
		}
	}
	return ""
}

func relatedByKind(graph graphView, from []string, kind string) []flowNode {
	seen := map[string]struct{}{}
	var out []flowNode
	for _, edge := range graph.edges {
		if !contains(from, edge.From) {
			continue
		}
		node, ok := graph.nodes[edge.To]
		if !ok {
			continue
		}
		if !strings.EqualFold(node.Kind, kind) {
			continue
		}
		if _, ok := seen[node.ID]; ok {
			continue
		}
		seen[node.ID] = struct{}{}
		out = append(out, node)
	}
	return out
}

func relatedPodsForServices(graph graphView, services []flowNode) []flowNode {
	serviceIDs := make([]string, 0, len(services))
	for _, svc := range services {
		serviceIDs = append(serviceIDs, svc.ID)
	}
	pods := relatedByKind(graph, serviceIDs, "Pod")
	endpoints := relatedByKind(graph, serviceIDs, "Endpoints")
	for _, ep := range endpoints {
		morePods := relatedByKind(graph, []string{ep.ID}, "Pod")
		pods = appendUnique(pods, morePods)
	}
	return uniqueNodes(pods)
}

func relatedWorkloadsForPods(graph graphView, pods []flowNode) []flowNode {
	podIDs := make([]string, 0, len(pods))
	for _, pod := range pods {
		podIDs = append(podIDs, pod.ID)
	}
	var owners []flowNode
	for _, edge := range graph.edges {
		if !contains(podIDs, edge.From) || edge.Relation != "owned-by" {
			continue
		}
		node, ok := graph.nodes[edge.To]
		if !ok {
			continue
		}
		if !isWorkloadKind(node.Kind) {
			continue
		}
		owners = append(owners, node)
	}
	return uniqueNodes(owners)
}

func policiesForPods(graph graphView, pods []flowNode) []flowNode {
	podIDs := make([]string, 0, len(pods))
	for _, pod := range pods {
		podIDs = append(podIDs, pod.ID)
	}
	var policies []flowNode
	for _, edge := range graph.edges {
		if contains(podIDs, edge.From) && strings.Contains(edge.Relation, "blocked") {
			if node, ok := graph.nodes[edge.To]; ok && strings.EqualFold(node.Kind, "NetworkPolicy") {
				policies = append(policies, node)
			}
		}
		if contains(podIDs, edge.To) && edge.Relation == "selects" {
			if node, ok := graph.nodes[edge.From]; ok && strings.EqualFold(node.Kind, "NetworkPolicy") {
				policies = append(policies, node)
			}
		}
	}
	return uniqueNodes(policies)
}

func podsFromEntry(graph graphView, entryID string) []flowNode {
	entry, ok := graph.nodes[entryID]
	if !ok {
		return nil
	}
	if strings.EqualFold(entry.Kind, "Pod") {
		return []flowNode{entry}
	}
	if strings.EqualFold(entry.Kind, "Service") {
		return relatedPodsForServices(graph, []flowNode{entry})
	}
	if isWorkloadKind(entry.Kind) {
		pods := relatedByKind(graph, []string{entry.ID}, "Pod")
		if len(pods) > 0 {
			return pods
		}
	}
	var pods []flowNode
	for _, node := range graph.nodes {
		if strings.EqualFold(node.Kind, "Pod") {
			pods = append(pods, node)
		}
	}
	return uniqueNodes(pods)
}

func hasKindGroup(graph graphView, kind, group string) bool {
	group = strings.ToLower(group)
	for _, node := range graph.nodes {
		if strings.EqualFold(node.Kind, kind) {
			if strings.Contains(strings.ToLower(node.ID), group) {
				return true
			}
		}
	}
	return false
}

func uniqueNodes(nodes []flowNode) []flowNode {
	seen := map[string]struct{}{}
	var out []flowNode
	for _, node := range nodes {
		if _, ok := seen[node.ID]; ok {
			continue
		}
		seen[node.ID] = struct{}{}
		out = append(out, node)
	}
	return out
}

func appendUnique(base []flowNode, more []flowNode) []flowNode {
	seen := map[string]struct{}{}
	for _, node := range base {
		seen[node.ID] = struct{}{}
	}
	for _, node := range more {
		if _, ok := seen[node.ID]; ok {
			continue
		}
		seen[node.ID] = struct{}{}
		base = append(base, node)
	}
	return base
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func isWorkloadKind(kind string) bool {
	switch strings.ToLower(kind) {
	case "deployment", "statefulset", "daemonset", "replicaset":
		return true
	default:
		return false
	}
}
