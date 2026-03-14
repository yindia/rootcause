package terraform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func (t *Toolset) handleDebugPlan(_ context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	plan, err := parsePlanInput(req.Arguments)
	if err != nil {
		return errorResult(err), err
	}
	analysis := render.NewAnalysis()

	resourceChanges := toSlice(plan["resource_changes"])
	resourceDrift := toSlice(plan["resource_drift"])
	outputChanges := toMap(plan["output_changes"])
	errored := toBool(plan["errored"])
	complete := true
	if raw, exists := plan["complete"]; exists {
		complete = toBool(raw)
	}

	includeNoOp := toBool(req.Arguments["includeNoOp"])
	focus := parseFocus(req.Arguments["focusAddresses"])
	providerSummary := map[string]map[string]int{}

	summary := map[string]int{
		"create":       0,
		"update":       0,
		"delete":       0,
		"replace":      0,
		"read":         0,
		"forget":       0,
		"no_op":        0,
		"unknownAfter": 0,
		"sensitive":    0,
	}

	for _, raw := range resourceChanges {
		change := toMap(raw)
		address := toString(change["address"])
		if !focus.match(address) {
			continue
		}
		provider := toString(change["provider_name"])
		changeObj := toMap(change["change"])
		actions := toStringSlice(changeObj["actions"])
		key := actionKey(actions)
		if key == "no_op" && !includeNoOp {
			continue
		}
		summary[key]++
		if hasReplace(actions) && key != "replace" {
			summary["replace"]++
		}
		if hasTrue(changeObj["after_unknown"]) {
			summary["unknownAfter"]++
		}
		if hasTrue(changeObj["after_sensitive"]) || hasTrue(changeObj["before_sensitive"]) {
			summary["sensitive"]++
		}
		if provider != "" {
			entry := providerSummary[provider]
			if entry == nil {
				entry = map[string]int{}
				providerSummary[provider] = entry
			}
			entry[key]++
			if hasReplace(actions) {
				entry["replace"]++
			}
		}

		severity := classifySeverity(actions)
		if severity != "" {
			reason := toString(change["action_reason"])
			details := fmt.Sprintf("%s (%s)", address, strings.Join(actions, ","))
			if reason != "" {
				details = fmt.Sprintf("%s reason=%s", details, reason)
			}
			analysis.AddCause("Plan change: "+severity, details, severity)
		}
		analysis.AddResource(address)
	}

	if len(resourceDrift) > 0 {
		analysis.AddCause("Resource drift detected", fmt.Sprintf("%d resources drifted from state", len(resourceDrift)), "high")
		analysis.AddEvidence("resourceDrift", resourceDrift)
	}

	if errored {
		analysis.AddCause("Plan errored", "Terraform marked this plan as errored; change list may be partial.", "high")
	}
	if !complete {
		analysis.AddCause("Plan incomplete", "Terraform marked this plan incomplete; additional planning may be required.", "medium")
	}

	analysis.AddEvidence("summary", summary)
	analysis.AddEvidence("resourceChangeCount", len(resourceChanges))
	analysis.AddEvidence("resourceDriftCount", len(resourceDrift))
	analysis.AddEvidence("outputChangeCount", len(outputChanges))
	if toBool(req.Arguments["summarizeByProvider"]) {
		analysis.AddEvidence("summaryByProvider", providerSummary)
	}

	analysis.AddNextCheck("Review replace/delete actions before apply.")
	analysis.AddNextCheck("Check unknown and sensitive fields to avoid hidden destructive changes.")
	analysis.AddNextCheck("If plan errored or incomplete, rerun `terraform plan` after fixing diagnostics.")

	return mcp.ToolResult{Data: t.ctx.Renderer.Render(analysis)}, nil
}

func parsePlanInput(args map[string]any) (map[string]any, error) {
	if rawPlan, ok := args["plan"]; ok {
		if planMap, ok := rawPlan.(map[string]any); ok {
			return planMap, nil
		}
	}
	planJSON := strings.TrimSpace(toString(args["planJSON"]))
	if planJSON == "" {
		return nil, errors.New("plan or planJSON is required")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(planJSON), &decoded); err != nil {
		return nil, fmt.Errorf("invalid planJSON: %w", err)
	}
	return decoded, nil
}

type focusSet map[string]struct{}

func parseFocus(raw any) focusSet {
	items := toStringSlice(raw)
	if len(items) == 0 {
		return nil
	}
	set := make(focusSet, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func (f focusSet) match(address string) bool {
	if len(f) == 0 {
		return true
	}
	_, ok := f[address]
	return ok
}

func actionKey(actions []string) string {
	if len(actions) == 0 {
		return "unknown"
	}
	if len(actions) == 1 {
		switch actions[0] {
		case "create", "update", "delete", "read", "forget":
			return actions[0]
		case "no-op":
			return "no_op"
		}
	}
	if hasReplace(actions) {
		return "replace"
	}
	return "unknown"
}

func hasReplace(actions []string) bool {
	if len(actions) != 2 {
		return false
	}
	return (actions[0] == "create" && actions[1] == "delete") || (actions[0] == "delete" && actions[1] == "create")
}

func classifySeverity(actions []string) string {
	if hasReplace(actions) {
		if len(actions) == 2 && actions[0] == "delete" {
			return "high"
		}
		return "medium"
	}
	if len(actions) == 1 {
		switch actions[0] {
		case "delete":
			return "high"
		case "update":
			return "medium"
		}
	}
	return ""
}

func hasTrue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case []any:
		for _, item := range v {
			if hasTrue(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range v {
			if hasTrue(item) {
				return true
			}
		}
	}
	return false
}

func toSlice(value any) []any {
	if out, ok := value.([]any); ok {
		return out
	}
	return nil
}

func toMap(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func toStringSlice(value any) []string {
	slice, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}
