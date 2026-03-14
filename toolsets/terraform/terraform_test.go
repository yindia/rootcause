package terraform

import (
	"context"
	"testing"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func TestRegisterIncludesTerraformTools(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	reg := mcp.NewRegistry(nil)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	expected := []string{
		"terraform.debug_plan",
		"terraform.list_modules",
		"terraform.get_module",
		"terraform.search_modules",
		"terraform.list_providers",
		"terraform.get_provider",
		"terraform.search_providers",
		"terraform.list_resources",
		"terraform.get_resource",
		"terraform.search_resources",
		"terraform.list_data_sources",
		"terraform.get_data_source",
		"terraform.search_data_sources",
	}
	for _, name := range expected {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("expected tool to be registered: %s", name)
		}
	}
}

func TestHandleDebugPlanSummarizesPlan(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{Renderer: render.NewRenderer()}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	plan := map[string]any{
		"complete": true,
		"resource_changes": []any{
			map[string]any{
				"address":       "aws_s3_bucket.logs",
				"provider_name": "registry.terraform.io/hashicorp/aws",
				"change": map[string]any{
					"actions":         []any{"create"},
					"after_unknown":   map[string]any{"id": true},
					"after_sensitive": map[string]any{},
				},
			},
			map[string]any{
				"address":       "aws_db_instance.main",
				"provider_name": "registry.terraform.io/hashicorp/aws",
				"action_reason": "replace_because_cannot_update",
				"change": map[string]any{
					"actions": []any{"delete", "create"},
					"after_sensitive": map[string]any{
						"password": true,
					},
				},
			},
		},
		"resource_drift": []any{map[string]any{"address": "aws_instance.old"}},
		"output_changes": map[string]any{"vpc_id": map[string]any{"actions": []any{"update"}}},
	}
	result, err := toolset.handleDebugPlan(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"plan": plan, "summarizeByProvider": true}})
	if err != nil {
		t.Fatalf("handleDebugPlan failed: %v", err)
	}
	root, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map result")
	}
	evidenceItems, ok := root["evidence"].([]render.EvidenceItem)
	if !ok {
		if generic, okCast := root["evidence"].([]any); okCast {
			evidenceItems = make([]render.EvidenceItem, 0, len(generic))
			for _, raw := range generic {
				if itemMap, okMap := raw.(map[string]any); okMap {
					evidenceItems = append(evidenceItems, render.EvidenceItem{
						Summary: toString(itemMap["summary"]),
						Details: itemMap["details"],
					})
				}
			}
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected evidence array")
	}
	summary := findEvidenceMap(t, evidenceItems, "summary")
	if got := toInt(summary["create"]); got != 1 {
		t.Fatalf("expected create=1, got=%d", got)
	}
	if got := toInt(summary["replace"]); got != 1 {
		t.Fatalf("expected replace=1, got=%d", got)
	}
	if got := toInt(summary["unknownAfter"]); got != 1 {
		t.Fatalf("expected unknownAfter=1, got=%d", got)
	}
	if got := toInt(summary["sensitive"]); got != 1 {
		t.Fatalf("expected sensitive=1, got=%d", got)
	}
}

func TestParsePlanInputRequiresInput(t *testing.T) {
	_, err := parsePlanInput(map[string]any{})
	if err == nil {
		t.Fatalf("expected error when no plan input is provided")
	}
}

func TestResolveProviderVersionInfoPrefersStableByDefault(t *testing.T) {
	payload := map[string]any{
		"included": []any{
			map[string]any{"id": "1", "type": "provider-versions", "attributes": map[string]any{"version": "1.2.0-beta.1"}},
			map[string]any{"id": "2", "type": "provider-versions", "attributes": map[string]any{"version": "1.1.0"}},
		},
	}
	id, latest := resolveProviderVersionInfo(payload, "", false)
	if latest != "1.1.0" {
		t.Fatalf("expected latest stable 1.1.0, got %s", latest)
	}
	if id != "2" {
		t.Fatalf("expected id 2 for latest stable, got %s", id)
	}
}

func findEvidenceMap(t *testing.T, items []render.EvidenceItem, key string) map[string]any {
	t.Helper()
	for _, item := range items {
		if item.Summary == key {
			if details, ok := item.Details.(map[string]any); ok {
				return details
			}
			if details, ok := item.Details.(map[string]int); ok {
				out := make(map[string]any, len(details))
				for k, v := range details {
					out[k] = v
				}
				return out
			}
			t.Fatalf("evidence %s did not contain a map", key)
		}
	}
	t.Fatalf("evidence %s not found", key)
	return nil
}
