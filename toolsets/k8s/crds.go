package k8s

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"rootcause/internal/mcp"
)

var crdGVR = schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}

func (t *Toolset) handleCRDs(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	if err := t.ctx.Policy.CheckNamespace(req.User, "", false); err != nil {
		return errorResult(err), err
	}
	query := strings.ToLower(toString(req.Arguments["query"]))
	var limit int
	if val, ok := req.Arguments["limit"].(float64); ok {
		limit = int(val)
	}

	list, err := t.ctx.Clients.Dynamic.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return errorResult(err), err
	}

	var items []map[string]any
	for i := range list.Items {
		info := crdSummary(&list.Items[i])
		if query != "" && !crdMatches(query, info) {
			continue
		}
		items = append(items, info)
		if limit > 0 && len(items) >= limit {
			break
		}
	}

	return mcp.ToolResult{Data: map[string]any{"crds": items, "matched": len(items)}}, nil
}

func crdSummary(obj *unstructured.Unstructured) map[string]any {
	info := map[string]any{"name": obj.GetName()}
	if group, ok, _ := unstructured.NestedString(obj.Object, "spec", "group"); ok {
		info["group"] = group
	}
	if scope, ok, _ := unstructured.NestedString(obj.Object, "spec", "scope"); ok {
		info["scope"] = scope
	}
	if kind, ok, _ := unstructured.NestedString(obj.Object, "spec", "names", "kind"); ok {
		info["kind"] = kind
	}
	if plural, ok, _ := unstructured.NestedString(obj.Object, "spec", "names", "plural"); ok {
		info["plural"] = plural
	}
	if shortNames, ok, _ := unstructured.NestedStringSlice(obj.Object, "spec", "names", "shortNames"); ok {
		info["shortNames"] = shortNames
	}
	if versions, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "versions"); ok {
		parsed := make([]map[string]any, 0, len(versions))
		for _, raw := range versions {
			entry := map[string]any{}
			if m, ok := raw.(map[string]any); ok {
				if name, ok := m["name"].(string); ok {
					entry["name"] = name
				}
				if served, ok := m["served"].(bool); ok {
					entry["served"] = served
				}
				if storage, ok := m["storage"].(bool); ok {
					entry["storage"] = storage
				}
			}
			if len(entry) > 0 {
				parsed = append(parsed, entry)
			}
		}
		if len(parsed) > 0 {
			info["versions"] = parsed
		}
	}
	return info
}

func crdMatches(query string, info map[string]any) bool {
	fields := []string{}
	if value, ok := info["name"].(string); ok {
		fields = append(fields, value)
	}
	if value, ok := info["group"].(string); ok {
		fields = append(fields, value)
	}
	if value, ok := info["kind"].(string); ok {
		fields = append(fields, value)
	}
	if value, ok := info["plural"].(string); ok {
		fields = append(fields, value)
	}
	if value, ok := info["scope"].(string); ok {
		fields = append(fields, value)
	}
	if shortNames, ok := info["shortNames"].([]string); ok {
		fields = append(fields, shortNames...)
	}
	if versions, ok := info["versions"].([]map[string]any); ok {
		for _, entry := range versions {
			if name, ok := entry["name"].(string); ok {
				fields = append(fields, name)
			}
		}
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
