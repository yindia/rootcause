package rootcause

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func (t *Toolset) handleCapabilities(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	includeSchemas := boolOrDefault(req.Arguments["includeSchemas"], false)
	tools := t.ctx.Registry.List()
	toolRows := make([]map[string]any, 0, len(tools))
	toolsets := map[string]struct{}{}
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		parts := strings.SplitN(name, ".", 2)
		if len(parts) > 0 && parts[0] != "" {
			toolsets[parts[0]] = struct{}{}
		}
		row := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if includeSchemas {
			row["inputSchema"] = tool.InputSchema
		}
		toolRows = append(toolRows, row)
	}
	deps := mcp.RequiredToolDependencies()
	edges := make([]map[string]any, 0)
	for _, dep := range deps {
		for _, required := range dep.Requires {
			edges = append(edges, map[string]any{"from": dep.Tool, "to": required, "source": "declared"})
		}
	}
	observedEdges := t.ctx.CallGraph.Edges()
	edges = append(edges, observedEdges...)
	toolsetNames := make([]string, 0, len(toolsets))
	for name := range toolsets {
		toolsetNames = append(toolsetNames, name)
	}
	sort.Strings(toolsetNames)
	out := map[string]any{
		"toolCount":         len(toolRows),
		"toolsets":          toolsetNames,
		"tools":             toolRows,
		"dependencyGraph":   map[string]any{"dependencies": deps, "edges": edges, "observedEdgeCount": len(observedEdges)},
		"dependencyVersion": "v2",
	}
	return mcp.ToolResult{Data: out}, nil
}

type bundleChainStep struct {
	Tool    string
	Section string
	Args    map[string]any
}

func (t *Toolset) handleIncidentBundle(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	namespace := toString(args["namespace"])
	keyword := toString(args["keyword"])
	eventLimit := intOrDefault(args["eventLimit"], 200)
	releaseLimit := intOrDefault(args["releaseLimit"], 50)
	timelineLimit := intOrDefault(args["timelineLimit"], 200)
	outputMode := strings.ToLower(strings.TrimSpace(toString(args["outputMode"])))
	if outputMode == "" {
		outputMode = "bundle"
	}
	includeHelm := boolOrDefault(args["includeHelm"], true)
	includeDefault := boolOrDefault(args["includeDefaultChain"], true)
	continueOnError := boolOrDefault(args["continueOnError"], true)
	maxSteps := intOrDefault(args["maxSteps"], 20)
	if maxSteps <= 0 {
		maxSteps = 20
	}

	sections := map[string]any{}
	errorsOut := make([]map[string]any, 0)
	executed := make([]map[string]any, 0)

	steps := make([]bundleChainStep, 0)
	if includeDefault {
		steps = append(steps, defaultBundleChain(namespace, keyword, eventLimit, releaseLimit, includeHelm)...)
	}
	if custom, ok := parseCustomChain(args["toolChain"]); ok {
		steps = append(steps, custom...)
	}
	if len(steps) > maxSteps {
		steps = steps[:maxSteps]
	}

	for _, step := range steps {
		if strings.TrimSpace(step.Tool) == "" {
			continue
		}
		if step.Tool == "rootcause.incident_bundle" {
			errorsOut = append(errorsOut, map[string]any{"tool": step.Tool, "error": "recursive rootcause.incident_bundle call is not allowed"})
			if !continueOnError {
				break
			}
			continue
		}
		data, err := t.call(ctx, req.User, step.Tool, step.Args)
		executed = append(executed, map[string]any{"tool": step.Tool, "section": step.Section, "ok": err == nil})
		if err != nil {
			errorsOut = append(errorsOut, map[string]any{"tool": step.Tool, "section": step.Section, "error": err.Error(), "data": data})
			if !continueOnError {
				break
			}
			continue
		}
		if step.Section == "" {
			step.Section = step.Tool
		}
		sections[step.Section] = data
	}

	bundle := map[string]any{
		"generatedAt":   nowRFC3339(),
		"namespace":     namespace,
		"keyword":       keyword,
		"outputMode":    outputMode,
		"timelineLimit": timelineLimit,
		"steps":         executed,
		"stepCount":     len(executed),
		"sections":      sections,
		"sectionCount":  len(sections),
		"errors":        errorsOut,
		"errorCount":    len(errorsOut),
	}

	if outputMode == "timeline" {
		return mcp.ToolResult{Data: buildTimelinePayloadFromBundle(bundle), Metadata: metadataForNamespace(namespace)}, nil
	}

	metadata := metadataForNamespace(namespace)
	return mcp.ToolResult{Data: bundle, Metadata: metadata}, nil
}

func metadataForNamespace(namespace string) mcp.ToolMetadata {
	metadata := mcp.ToolMetadata{}
	if namespace != "" {
		metadata.Namespaces = []string{namespace}
	}
	return metadata
}

func defaultBundleChain(namespace, keyword string, eventLimit, releaseLimit int, includeHelm bool) []bundleChainStep {
	steps := make([]bundleChainStep, 0)
	overviewArgs := map[string]any{}
	if namespace != "" {
		overviewArgs["namespace"] = namespace
	}
	steps = append(steps, bundleChainStep{Tool: "k8s.overview", Section: "overview", Args: overviewArgs})

	eventArgs := map[string]any{"limit": eventLimit}
	if namespace != "" {
		eventArgs["namespace"] = namespace
	}
	steps = append(steps, bundleChainStep{Tool: "k8s.events_timeline", Section: "eventsTimeline", Args: eventArgs})

	if keyword != "" {
		diagArgs := map[string]any{"keyword": keyword, "autoFlow": true}
		if namespace != "" {
			diagArgs["namespace"] = namespace
		}
		steps = append(steps, bundleChainStep{Tool: "k8s.diagnose", Section: "diagnose", Args: diagArgs})
	}

	if includeHelm {
		helmArgs := map[string]any{"limit": releaseLimit}
		if namespace != "" {
			helmArgs["namespace"] = namespace
		}
		steps = append(steps, bundleChainStep{Tool: "helm.list", Section: "helmReleases", Args: helmArgs})
	}
	return steps
}

func parseCustomChain(value any) ([]bundleChainStep, bool) {
	raw, ok := value.([]any)
	if !ok || len(raw) == 0 {
		return nil, false
	}
	steps := make([]bundleChainStep, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tool := strings.TrimSpace(toString(entry["tool"]))
		if tool == "" {
			continue
		}
		step := bundleChainStep{Tool: tool, Section: strings.TrimSpace(toString(entry["section"])), Args: map[string]any{}}
		if args, ok := entry["args"].(map[string]any); ok {
			step.Args = args
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, false
	}
	return steps, true
}

func (t *Toolset) call(ctx context.Context, user policy.User, tool string, args map[string]any) (any, error) {
	if t.ctx.Invoker == nil {
		return map[string]any{"error": "tool invoker not available"}, fmt.Errorf("tool invoker not available")
	}
	result, err := t.ctx.CallTool(ctx, user, tool, args)
	if err != nil {
		return result.Data, err
	}
	return result.Data, nil
}

func (t *Toolset) handleRCAGenerate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	bundleRaw := args["bundle"]
	if bundleRaw == nil {
		bundleArgs := map[string]any{}
		if ns := toString(args["namespace"]); ns != "" {
			bundleArgs["namespace"] = ns
		}
		if keyword := toString(args["keyword"]); keyword != "" {
			bundleArgs["keyword"] = keyword
		}
		bundleData, err := t.call(ctx, req.User, "rootcause.incident_bundle", bundleArgs)
		if err != nil {
			return mcp.ToolResult{Data: map[string]any{"error": err.Error(), "bundle": bundleData}}, err
		}
		bundleRaw = bundleData
	}
	bundle, ok := bundleRaw.(map[string]any)
	if !ok {
		err := fmt.Errorf("bundle must be an object")
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	sections, _ := bundle["sections"].(map[string]any)
	errorCount := intOrDefault(bundle["errorCount"], 0)

	rootCauses := make([]string, 0)
	if diagnose, ok := sections["diagnose"].(map[string]any); ok {
		if causes, ok := diagnose["likelyRootCauses"].([]any); ok {
			for _, raw := range causes {
				if cause, ok := raw.(map[string]any); ok {
					s := strings.TrimSpace(toString(cause["summary"]))
					if s != "" {
						rootCauses = append(rootCauses, s)
					}
				}
			}
		}
	}
	if len(rootCauses) == 0 {
		rootCauses = append(rootCauses, "Insufficient direct cause evidence; review event timeline and workload logs.")
	}

	recommendations := []string{
		"Stabilize affected workloads before attempting wider changes.",
		"Apply targeted fix and verify with k8s.events_timeline + service health checks.",
		"Document prevention actions (alerts, runbooks, rollout guardrails).",
	}
	if errorCount > 0 {
		recommendations = append(recommendations, "Resolve data collection errors and regenerate RCA for higher confidence.")
	}

	incidentSummary := strings.TrimSpace(toString(args["incidentSummary"]))
	if incidentSummary == "" {
		incidentSummary = "Automated RCA draft generated from collected bundle evidence."
	}
	rca := map[string]any{
		"title":              "Root Cause Analysis Draft",
		"incidentSummary":    incidentSummary,
		"rootCauses":         rootCauses,
		"evidenceReferences": map[string]any{"sectionCount": len(sections), "bundleGeneratedAt": bundle["generatedAt"]},
		"recommendations":    recommendations,
		"confidence":         confidenceLabel(errorCount),
	}
	return mcp.ToolResult{Data: map[string]any{"rca": rca, "bundle": bundle}}, nil
}

func confidenceLabel(errorCount int) string {
	if errorCount == 0 {
		return "high"
	}
	if errorCount <= 2 {
		return "medium"
	}
	return "low"
}

func (t *Toolset) handleRemediationPlaybook(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	bundle, err := t.resolveBundle(ctx, req, args)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	rca, err := t.resolveRCA(ctx, req, args, bundle)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error(), "bundle": bundle}}, err
	}
	maxImmediate := intOrDefault(args["maxImmediateActions"], 3)
	if maxImmediate <= 0 {
		maxImmediate = 3
	}
	rootCauses := toStringList(rca["rootCauses"])
	recommendations := toStringList(rca["recommendations"])
	immediate := make([]map[string]any, 0)
	for idx, cause := range rootCauses {
		if idx >= maxImmediate {
			break
		}
		immediate = append(immediate, map[string]any{
			"priority":  idx + 1,
			"title":     fmt.Sprintf("Stabilize issue: %s", cause),
			"owner":     "oncall",
			"toolHints": []string{"k8s.events_timeline", "k8s.restart_safety_check", "helm.diff_release"},
		})
	}
	followUp := make([]map[string]any, 0)
	for _, rec := range recommendations {
		followUp = append(followUp, map[string]any{
			"title": rec,
			"owner": "platform",
		})
	}
	sort.Slice(followUp, func(i, j int) bool {
		return toString(followUp[i]["title"]) < toString(followUp[j]["title"])
	})
	playbook := map[string]any{
		"title":            "Incident Remediation Playbook",
		"generatedAt":      nowRFC3339(),
		"namespace":        bundle["namespace"],
		"immediateActions": immediate,
		"followUpActions":  followUp,
		"validation": []string{
			"Confirm error rate and latency return to baseline.",
			"Verify event timeline has no new warning spikes.",
			"Run smoke tests on critical user journeys.",
		},
	}
	return mcp.ToolResult{Data: map[string]any{"playbook": playbook, "bundle": bundle, "rca": rca}}, nil
}

func (t *Toolset) handlePostmortemExport(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := req.Arguments
	bundle, err := t.resolveBundle(ctx, req, args)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error()}}, err
	}
	rca, err := t.resolveRCA(ctx, req, args, bundle)
	if err != nil {
		return mcp.ToolResult{Data: map[string]any{"error": err.Error(), "bundle": bundle}}, err
	}
	incidentSummary := strings.TrimSpace(toString(args["incidentSummary"]))
	if incidentSummary == "" {
		incidentSummary = toString(rca["incidentSummary"])
	}
	if incidentSummary == "" {
		incidentSummary = "Incident summary pending detailed write-up."
	}
	doc := map[string]any{
		"title":             "Postmortem",
		"generatedAt":       nowRFC3339(),
		"incidentSummary":   incidentSummary,
		"timelineSection":   bundle["sections"],
		"rootCauses":        rca["rootCauses"],
		"recommendations":   rca["recommendations"],
		"confidence":        rca["confidence"],
		"actionItems":       toStringList(rca["recommendations"]),
		"bundleErrorCount":  bundle["errorCount"],
		"bundleGeneratedAt": bundle["generatedAt"],
	}
	format := strings.ToLower(strings.TrimSpace(toString(args["format"])))
	if format == "" {
		format = "json"
	}
	if format == "markdown" {
		return mcp.ToolResult{Data: map[string]any{"format": "markdown", "content": renderPostmortemMarkdown(doc), "document": doc}}, nil
	}
	return mcp.ToolResult{Data: map[string]any{"format": "json", "document": doc}}, nil
}

func (t *Toolset) resolveBundle(ctx context.Context, req mcp.ToolRequest, args map[string]any) (map[string]any, error) {
	bundleRaw := args["bundle"]
	if bundleRaw == nil {
		bundleArgs := map[string]any{}
		if ns := toString(args["namespace"]); ns != "" {
			bundleArgs["namespace"] = ns
		}
		if keyword := toString(args["keyword"]); keyword != "" {
			bundleArgs["keyword"] = keyword
		}
		bundleData, err := t.call(ctx, req.User, "rootcause.incident_bundle", bundleArgs)
		if err != nil {
			if typed, ok := bundleData.(map[string]any); ok {
				return typed, err
			}
			return map[string]any{"error": toString(bundleData)}, err
		}
		bundleRaw = bundleData
	}
	bundle, ok := bundleRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("bundle must be an object")
	}
	return bundle, nil
}

func (t *Toolset) resolveRCA(ctx context.Context, req mcp.ToolRequest, args map[string]any, bundle map[string]any) (map[string]any, error) {
	rcaRaw := args["rca"]
	if rcaRaw == nil {
		rcaData, err := t.call(ctx, req.User, "rootcause.rca_generate", map[string]any{
			"bundle":          bundle,
			"incidentSummary": args["incidentSummary"],
		})
		if err != nil {
			return nil, err
		}
		rcaContainer, ok := rcaData.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid rca_generate response")
		}
		rcaRaw = rcaContainer["rca"]
	}
	rca, ok := rcaRaw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("rca must be an object")
	}
	return rca, nil
}

func toStringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		if list, ok := value.([]string); ok {
			return append([]string{}, list...)
		}
		return []string{}
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text := strings.TrimSpace(toString(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func renderPostmortemMarkdown(doc map[string]any) string {
	title := toString(doc["title"])
	incidentSummary := toString(doc["incidentSummary"])
	rootCauses := toStringList(doc["rootCauses"])
	recommendations := toStringList(doc["recommendations"])
	b := &strings.Builder{}
	b.WriteString("# " + title + "\n\n")
	b.WriteString("## Incident Summary\n")
	b.WriteString(incidentSummary + "\n\n")
	b.WriteString("## Root Causes\n")
	for _, item := range rootCauses {
		b.WriteString("- " + item + "\n")
	}
	b.WriteString("\n## Recommendations\n")
	for _, item := range recommendations {
		b.WriteString("- " + item + "\n")
	}
	return b.String()
}

func (t *Toolset) handleChangeTimeline(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	args := cloneMap(req.Arguments)
	args["outputMode"] = "timeline"
	return t.handleIncidentBundle(ctx, mcp.ToolRequest{User: req.User, Arguments: args})
}

func buildTimelinePayloadFromBundle(bundle map[string]any) map[string]any {
	sections, _ := bundle["sections"].(map[string]any)
	keyword := strings.TrimSpace(toString(bundle["keyword"]))
	timeline := make([]map[string]any, 0)
	if eventsPayload, ok := sections["eventsTimeline"]; ok {
		timeline = append(timeline, timelineFromEvents(eventsPayload, keyword)...)
	}
	if helmPayload, ok := sections["helmReleases"]; ok {
		timeline = append(timeline, timelineFromHelmReleases(helmPayload, keyword)...)
	}
	sort.Slice(timeline, func(i, j int) bool {
		return parseTimelineTime(timeline[i]["time"]).Before(parseTimelineTime(timeline[j]["time"]))
	})
	timelineLimit := intOrDefault(bundle["timelineLimit"], 0)
	if timelineLimit > 0 && len(timeline) > timelineLimit {
		timeline = timeline[len(timeline)-timelineLimit:]
	}
	out := map[string]any{
		"namespace":     bundle["namespace"],
		"keyword":       bundle["keyword"],
		"timeline":      timeline,
		"timelineCount": len(timeline),
		"errorCount":    bundle["errorCount"],
		"errors":        bundle["errors"],
		"steps":         bundle["steps"],
		"stepCount":     bundle["stepCount"],
	}
	if len(timeline) > 0 {
		out["startTime"] = timeline[0]["time"]
		out["endTime"] = timeline[len(timeline)-1]["time"]
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func timelineFromEvents(payload any, keyword string) []map[string]any {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	rawTimeline, ok := root["timeline"].([]map[string]any)
	if !ok {
		if generic, okAny := root["timeline"].([]any); okAny {
			converted := make([]map[string]any, 0, len(generic))
			for _, item := range generic {
				if entry, okMap := item.(map[string]any); okMap {
					converted = append(converted, entry)
				}
			}
			rawTimeline = converted
		}
	}
	out := make([]map[string]any, 0, len(rawTimeline))
	for _, item := range rawTimeline {
		message := toString(item["message"])
		reason := toString(item["reason"])
		if keyword != "" && !strings.Contains(strings.ToLower(message+" "+reason), strings.ToLower(keyword)) {
			continue
		}
		obj, _ := item["object"].(map[string]any)
		out = append(out, map[string]any{
			"time":     normalizeTimeString(item["time"]),
			"source":   "k8s.event",
			"severity": strings.ToLower(toString(item["type"])),
			"summary":  fmt.Sprintf("%s: %s", reason, message),
			"resource": fmt.Sprintf("%s/%s", toString(obj["kind"]), toString(obj["name"])),
			"raw":      item,
		})
	}
	return out
}

func timelineFromHelmReleases(payload any, keyword string) []map[string]any {
	root, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	rawReleases, ok := root["releases"].([]any)
	if !ok {
		if typed, okTyped := root["releases"].([]map[string]any); okTyped {
			rawReleases = make([]any, 0, len(typed))
			for _, item := range typed {
				rawReleases = append(rawReleases, item)
			}
		}
	}
	out := make([]map[string]any, 0, len(rawReleases))
	for _, item := range rawReleases {
		rel, okMap := item.(map[string]any)
		if !okMap {
			continue
		}
		name := toString(rel["name"])
		status := toString(rel["status"])
		if keyword != "" && !strings.Contains(strings.ToLower(name+" "+status), strings.ToLower(keyword)) {
			continue
		}
		out = append(out, map[string]any{
			"time":     normalizeTimeString(rel["updated"]),
			"source":   "helm.release",
			"severity": helmStatusSeverity(status),
			"summary":  fmt.Sprintf("Release %s status=%s revision=%v", name, status, rel["revision"]),
			"resource": fmt.Sprintf("HelmRelease/%s", name),
			"raw":      rel,
		})
	}
	return out
}

func parseTimelineTime(value any) time.Time {
	if typed, ok := value.(time.Time); ok {
		return typed.UTC()
	}
	raw := strings.TrimSpace(toString(value))
	if raw == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func normalizeTimeString(value any) string {
	parsed := parseTimelineTime(value)
	if parsed.IsZero() {
		return ""
	}
	return parsed.Format(time.RFC3339)
}

func helmStatusSeverity(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "failed", "pending-install", "pending-upgrade", "pending-rollback":
		return "high"
	case "uninstalled", "superseded", "uninstalling":
		return "medium"
	default:
		return "low"
	}
}
