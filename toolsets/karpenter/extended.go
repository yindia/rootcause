package karpenter

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/render"
)

type resourceMatch struct {
	GVR        schema.GroupVersionResource
	Kind       string
	Namespaced bool
	Group      string
	Version    string
	Resource   string
	ShortNames []string
}

type nodeClassIndex struct {
	byKind map[string]map[string]struct{}
	byName map[string][]string
}

func (t *Toolset) handleNodePoolDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, _, groups, err := t.detectKarpenter(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "karpenter not detected")
		analysis.AddEvidence("groupsChecked", karpenterGroups)
		analysis.AddNextCheck("Install Karpenter CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}

	namespace := toString(req.Arguments["namespace"])
	name := toString(req.Arguments["name"])
	selector := toString(req.Arguments["labelSelector"])

	var classIndex *nodeClassIndex
	if req.User.Role == policy.RoleCluster {
		index, err := t.buildNodeClassIndex(ctx, req.User)
		if err != nil {
			analysis.AddEvidence("nodeClassLookupError", err.Error())
		} else {
			classIndex = index
		}
	} else {
		analysis.AddEvidence("nodeClassLookup", "requires cluster role")
	}

	matches, err := t.findResourcesByKind(func(kind string) bool {
		return strings.EqualFold(kind, "NodePool") || strings.EqualFold(kind, "Provisioner")
	}, func(group string) bool {
		return group == "karpenter.sh"
	})
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if len(matches) == 0 {
		analysis.AddEvidence("status", "no NodePool or Provisioner resources found")
		analysis.AddNextCheck("Install NodePool/Provisioner CRDs")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}

	for _, match := range matches {
		objects, namespaces, err := t.listResourceObjects(ctx, req.User, match, namespace, name, selector)
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		if len(objects) == 0 {
			continue
		}
		analysis.AddEvidence(fmt.Sprintf("%sCount", match.Kind), len(objects))
		for i := range objects {
			obj := &objects[i]
			ref := t.ctx.Evidence.ResourceRef(match.GVR, obj.GetNamespace(), obj.GetName())
			analysis.AddResource(ref)
			nodeClassRef := selectNodeClassRef(obj)
			nodeClassResolution := t.resolveNodeClassRef(nodeClassRef, classIndex)
			conditions := extractConditions(obj)
			analysis.AddEvidence(fmt.Sprintf("%s %s", match.Kind, obj.GetName()), map[string]any{
				"requirements":  extractRequirements(obj),
				"taints":        extractTaints(obj),
				"limits":        extractLimits(obj),
				"disruption":    extractDisruption(obj),
				"weight":        nestedInt(obj, "spec", "weight"),
				"nodeClassRef":  nodeClassRef,
				"nodeClass":     nodeClassResolution,
				"providerRef":   extractProviderRef(obj),
				"conditions":    conditions,
				"templateClass": extractNodeClassRefFromTemplate(obj),
			})
			for _, cond := range conditions {
				if isConditionFalse(cond, []string{"Ready"}) {
					analysis.AddCause("NodePool not ready", fmt.Sprintf("%s %s condition %s false", match.Kind, obj.GetName(), cond["type"]), "high")
				}
			}
			if nodeClassResolution != nil {
				if found, ok := nodeClassResolution["found"].(bool); ok && !found {
					target := fmt.Sprintf("%v", nodeClassResolution["name"])
					analysis.AddCause("NodeClass missing", fmt.Sprintf("%s %s references missing NodeClass %s", match.Kind, obj.GetName(), target), "high")
				}
			}
		}
		if len(namespaces) > 0 {
			analysis.AddEvidence(fmt.Sprintf("%sNamespaces", match.Kind), namespaces)
		}
	}

	analysis.AddNextCheck("Verify NodeClass references and Karpenter controller logs")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleNodeClassDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, _, groups, err := t.detectKarpenter(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "karpenter not detected")
		analysis.AddEvidence("groupsChecked", karpenterGroups)
		analysis.AddNextCheck("Install Karpenter CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}

	namespace := toString(req.Arguments["namespace"])
	name := toString(req.Arguments["name"])
	kindFilter := toString(req.Arguments["kind"])
	selector := toString(req.Arguments["labelSelector"])

	matches, err := t.findResourcesByKind(func(kind string) bool {
		if kindFilter != "" && !strings.EqualFold(kind, kindFilter) {
			return false
		}
		return strings.HasSuffix(kind, "NodeClass")
	}, func(group string) bool {
		return strings.Contains(group, "karpenter")
	})
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if len(matches) == 0 {
		analysis.AddEvidence("status", "no NodeClass resources found")
		analysis.AddNextCheck("Install Karpenter provider NodeClass CRDs")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}

	for _, match := range matches {
		objects, namespaces, err := t.listResourceObjects(ctx, req.User, match, namespace, name, selector)
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for i := range objects {
			obj := &objects[i]
			ref := t.ctx.Evidence.ResourceRef(match.GVR, obj.GetNamespace(), obj.GetName())
			analysis.AddResource(ref)
			spec, _ := nestedMap(obj, "spec")
			conditions := extractConditions(obj)
			analysis.AddEvidence(fmt.Sprintf("%s %s", match.Kind, obj.GetName()), map[string]any{
				"specKeys":   mapKeys(spec),
				"spec":       t.ctx.Redactor.RedactMap(spec),
				"conditions": conditions,
			})
			t.addAWSNodeClassEvidence(ctx, req, &analysis, match, obj)
			for _, cond := range conditions {
				if isConditionFalse(cond, []string{"Ready"}) {
					analysis.AddCause("NodeClass not ready", fmt.Sprintf("%s %s condition %s false", match.Kind, obj.GetName(), cond["type"]), "high")
				}
			}
		}
		if len(namespaces) > 0 {
			analysis.AddEvidence(fmt.Sprintf("%sNamespaces", match.Kind), namespaces)
		}
	}

	analysis.AddNextCheck("Verify NodeClass fields and provider permissions")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) handleInterruptionDebug(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	analysis := render.NewAnalysis()
	detected, _, groups, err := t.detectKarpenter(ctx)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if !detected {
		analysis.AddEvidence("status", "karpenter not detected")
		analysis.AddEvidence("groupsChecked", karpenterGroups)
		analysis.AddNextCheck("Install Karpenter CRDs or verify API group availability")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}
	if len(groups) > 0 {
		analysis.AddEvidence("groupsFound", groups)
	}

	namespace := toString(req.Arguments["namespace"])
	name := toString(req.Arguments["name"])
	selector := toString(req.Arguments["labelSelector"])

	matches, err := t.findResourcesByKind(func(kind string) bool {
		return strings.EqualFold(kind, "NodeClaim")
	}, func(group string) bool {
		return group == "karpenter.sh"
	})
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	if len(matches) == 0 {
		analysis.AddEvidence("status", "no NodeClaim resources found")
		analysis.AddNextCheck("Verify Karpenter NodeClaim CRDs")
		return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
	}

	for _, match := range matches {
		objects, namespaces, err := t.listResourceObjects(ctx, req.User, match, namespace, name, selector)
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
		}
		for i := range objects {
			obj := &objects[i]
			ref := t.ctx.Evidence.ResourceRef(match.GVR, obj.GetNamespace(), obj.GetName())
			analysis.AddResource(ref)
			conditions := extractConditions(obj)
			nodeName := nestedString(obj, "status", "nodeName")
			analysis.AddEvidence(fmt.Sprintf("NodeClaim %s", obj.GetName()), map[string]any{
				"nodeName":     nodeName,
				"providerID":   nestedString(obj, "status", "providerID"),
				"nodePool":     nestedString(obj, "spec", "nodePool"),
				"nodeClassRef": extractNodeClassRef(obj),
				"requirements": extractRequirements(obj),
				"conditions":   conditions,
			})
			for _, cond := range conditions {
				if isConditionFalse(cond, []string{"Ready", "Initialized", "Launched"}) {
					analysis.AddCause("NodeClaim not ready", fmt.Sprintf("NodeClaim %s condition %s false", obj.GetName(), cond["type"]), "high")
				}
				if isConditionTrue(cond, []string{"Drifted", "DisruptionBlocked", "Expired"}) {
					analysis.AddCause("NodeClaim disruption", fmt.Sprintf("NodeClaim %s condition %s true", obj.GetName(), cond["type"]), "medium")
				}
			}
			if nodeName != "" && req.User.Role == policy.RoleCluster {
				node, err := t.ctx.Clients.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				if err == nil {
					analysis.AddEvidence(fmt.Sprintf("Node %s", nodeName), map[string]any{
						"conditions": node.Status.Conditions,
						"taints":     node.Spec.Taints,
						"labels":     node.Labels,
					})
				}
			}
		}
		if len(namespaces) > 0 {
			analysis.AddEvidence("nodeClaimNamespaces", namespaces)
		}
	}

	analysis.AddNextCheck("Review NodeClaim conditions and interruption handling logs")
	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis), Metadata: mcp.ToolMetadata{Namespaces: sliceIf(namespace)}}, nil
}

func (t *Toolset) findResourcesByKind(kindMatch func(string) bool, groupMatch func(string) bool) ([]resourceMatch, error) {
	lists, err := t.ctx.Clients.Discovery.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, err
	}
	var resources []resourceMatch
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		if groupMatch != nil && !groupMatch(gv.Group) {
			continue
		}
		for _, res := range list.APIResources {
			if res.Name == "" || strings.Contains(res.Name, "/") {
				continue
			}
			if kindMatch != nil && !kindMatch(res.Kind) {
				continue
			}
			resources = append(resources, resourceMatch{
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

func (t *Toolset) buildNodeClassIndex(ctx context.Context, user policy.User) (*nodeClassIndex, error) {
	matches, err := t.findResourcesByKind(func(kind string) bool {
		return strings.HasSuffix(kind, "NodeClass")
	}, func(group string) bool {
		return strings.Contains(group, "karpenter")
	})
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	index := &nodeClassIndex{byKind: map[string]map[string]struct{}{}, byName: map[string][]string{}}
	for _, match := range matches {
		items, _, err := t.listResourceObjects(ctx, user, match, "", "", "")
		if err != nil {
			return nil, err
		}
		for i := range items {
			obj := &items[i]
			kindKey := strings.ToLower(match.Kind)
			if _, ok := index.byKind[kindKey]; !ok {
				index.byKind[kindKey] = map[string]struct{}{}
			}
			index.byKind[kindKey][obj.GetName()] = struct{}{}
			index.byName[obj.GetName()] = append(index.byName[obj.GetName()], match.Kind)
		}
	}
	for name := range index.byName {
		sort.Strings(index.byName[name])
	}
	return index, nil
}

func selectNodeClassRef(obj *unstructured.Unstructured) map[string]any {
	if ref := extractNodeClassRef(obj); len(ref) > 0 {
		return ref
	}
	if ref := extractNodeClassRefFromTemplate(obj); len(ref) > 0 {
		return ref
	}
	return nil
}

func (t *Toolset) resolveNodeClassRef(ref map[string]any, index *nodeClassIndex) map[string]any {
	if ref == nil || index == nil {
		return nil
	}
	name := toString(ref["name"])
	if name == "" {
		return map[string]any{"found": false, "reason": "missing name"}
	}
	kind := toString(ref["kind"])
	if kind != "" {
		kindKey := strings.ToLower(kind)
		if names, ok := index.byKind[kindKey]; ok {
			if _, exists := names[name]; exists {
				return map[string]any{"found": true, "name": name, "kind": kind}
			}
		}
		return map[string]any{"found": false, "name": name, "kind": kind}
	}
	if kinds, ok := index.byName[name]; ok && len(kinds) > 0 {
		return map[string]any{"found": true, "name": name, "kinds": kinds}
	}
	return map[string]any{"found": false, "name": name}
}

func (t *Toolset) listResourceObjects(ctx context.Context, user policy.User, match resourceMatch, namespace, name, selector string) ([]unstructured.Unstructured, []string, error) {
	opts := metav1.ListOptions{LabelSelector: selector}
	if match.Namespaced {
		if namespace != "" {
			if err := t.ctx.Policy.CheckNamespace(user, namespace, true); err != nil {
				return nil, nil, err
			}
			if name != "" {
				obj, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil, nil, nil
					}
					return nil, nil, err
				}
				return []unstructured.Unstructured{*obj}, []string{namespace}, nil
			}
			list, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(namespace).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			return list.Items, []string{namespace}, nil
		}
		if user.Role == policy.RoleCluster {
			if name != "" {
				list, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(metav1.NamespaceAll).List(ctx, opts)
				if err != nil {
					return nil, nil, err
				}
				filtered := filterByName(list.Items, name)
				return filtered, nil, nil
			}
			list, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(metav1.NamespaceAll).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			return list.Items, nil, nil
		}
		namespaces, err := t.allowedNamespaces(ctx, user, namespace)
		if err != nil {
			return nil, nil, err
		}
		var items []unstructured.Unstructured
		for _, ns := range namespaces {
			if err := t.ctx.Policy.CheckNamespace(user, ns, true); err != nil {
				return nil, nil, err
			}
			if name != "" {
				obj, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					continue
				}
				if err != nil {
					return nil, nil, err
				}
				items = append(items, *obj)
				continue
			}
			list, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Namespace(ns).List(ctx, opts)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, list.Items...)
		}
		if name != "" && len(items) > 1 {
			return nil, nil, fmt.Errorf("resource %q found in multiple namespaces", name)
		}
		return items, namespaces, nil
	}
	if err := t.ctx.Policy.CheckNamespace(user, "", false); err != nil {
		return nil, nil, err
	}
	if name != "" {
		obj, err := t.ctx.Clients.Dynamic.Resource(match.GVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil, nil
			}
			return nil, nil, err
		}
		return []unstructured.Unstructured{*obj}, nil, nil
	}
	list, err := t.ctx.Clients.Dynamic.Resource(match.GVR).List(ctx, opts)
	if err != nil {
		return nil, nil, err
	}
	return list.Items, nil, nil
}

func filterByName(items []unstructured.Unstructured, name string) []unstructured.Unstructured {
	if name == "" {
		return items
	}
	var out []unstructured.Unstructured
	for _, item := range items {
		if item.GetName() == name {
			out = append(out, item)
		}
	}
	return out
}

func extractConditions(obj *unstructured.Unstructured) []map[string]any {
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

func extractRequirements(obj *unstructured.Unstructured) []map[string]any {
	for _, path := range [][]string{
		{"spec", "template", "spec", "requirements"},
		{"spec", "requirements"},
	} {
		items, found, _ := unstructured.NestedSlice(obj.Object, path...)
		if !found {
			continue
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			req, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, map[string]any{
				"key":      req["key"],
				"operator": req["operator"],
				"values":   req["values"],
				"minValues": func() any {
					if value, ok := req["minValues"]; ok {
						return value
					}
					return nil
				}(),
			})
		}
		return out
	}
	return nil
}

func extractTaints(obj *unstructured.Unstructured) []map[string]any {
	for _, path := range [][]string{
		{"spec", "template", "spec", "taints"},
		{"spec", "taints"},
	} {
		items, found, _ := unstructured.NestedSlice(obj.Object, path...)
		if !found {
			continue
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			taint, ok := item.(map[string]any)
			if !ok {
				continue
			}
			out = append(out, map[string]any{
				"key":    taint["key"],
				"value":  taint["value"],
				"effect": taint["effect"],
			})
		}
		return out
	}
	return nil
}

func extractLimits(obj *unstructured.Unstructured) map[string]any {
	if limits, ok := nestedMap(obj, "spec", "limits", "resources"); ok {
		return limits
	}
	if limits, ok := nestedMap(obj, "spec", "limits"); ok {
		return limits
	}
	return nil
}

func extractDisruption(obj *unstructured.Unstructured) map[string]any {
	if disruption, ok := nestedMap(obj, "spec", "disruption"); ok {
		return disruption
	}
	out := map[string]any{}
	if ttl := nestedInt(obj, "spec", "ttlSecondsAfterEmpty"); ttl != 0 {
		out["ttlSecondsAfterEmpty"] = ttl
	}
	if ttl := nestedInt(obj, "spec", "ttlSecondsUntilExpired"); ttl != 0 {
		out["ttlSecondsUntilExpired"] = ttl
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractNodeClassRef(obj *unstructured.Unstructured) map[string]any {
	if ref, ok := nestedMap(obj, "spec", "nodeClassRef"); ok {
		return ref
	}
	return nil
}

func extractNodeClassRefFromTemplate(obj *unstructured.Unstructured) map[string]any {
	if ref, ok := nestedMap(obj, "spec", "template", "spec", "nodeClassRef"); ok {
		return ref
	}
	return nil
}

func extractProviderRef(obj *unstructured.Unstructured) map[string]any {
	if ref, ok := nestedMap(obj, "spec", "providerRef"); ok {
		return ref
	}
	return nil
}

func nestedMap(obj *unstructured.Unstructured, fields ...string) (map[string]any, bool) {
	if obj == nil {
		return nil, false
	}
	value, ok, _ := unstructured.NestedMap(obj.Object, fields...)
	return value, ok
}

func nestedString(obj *unstructured.Unstructured, fields ...string) string {
	if obj == nil {
		return ""
	}
	value, _, _ := unstructured.NestedString(obj.Object, fields...)
	return value
}

func nestedInt(obj *unstructured.Unstructured, fields ...string) int64 {
	if obj == nil {
		return 0
	}
	value, _, _ := unstructured.NestedInt64(obj.Object, fields...)
	return value
}

func mapKeys(input map[string]any) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isConditionFalse(cond map[string]any, types []string) bool {
	if cond == nil {
		return false
	}
	typ, ok := cond["type"].(string)
	if !ok {
		return false
	}
	status := fmt.Sprintf("%v", cond["status"])
	if status != string(corev1.ConditionFalse) && status != "False" {
		return false
	}
	for _, t := range types {
		if typ == t {
			return true
		}
	}
	return false
}

func isConditionTrue(cond map[string]any, types []string) bool {
	if cond == nil {
		return false
	}
	typ, ok := cond["type"].(string)
	if !ok {
		return false
	}
	status := fmt.Sprintf("%v", cond["status"])
	if status != string(corev1.ConditionTrue) && status != "True" {
		return false
	}
	for _, t := range types {
		if typ == t {
			return true
		}
	}
	return false
}
