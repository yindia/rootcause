package observability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"rootcause/internal/mcp"
)

func (t *Toolset) handleLogsQuery(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	filter := strings.TrimSpace(argString(req.Arguments, "filter"))
	if filter == "" {
		err := fmt.Errorf("filter is required")
		return errorResult(err), err
	}
	window := parseDuration(argString(req.Arguments, "duration"), 30*time.Minute)
	limit := argInt(req.Arguments, "limit", 100)
	res, err := backend.Logs().RawQuery(ctx, project, filter, window, limit)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"backend": backend.Name(),
		"project": res.Project,
		"filter":  res.Filter,
		"entries": res.Entries,
		"count":   len(res.Entries),
	}}, nil
}

func (t *Toolset) handleLogsWorkload(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	namespace := strings.TrimSpace(argString(req.Arguments, "namespace"))
	workload := strings.TrimSpace(argString(req.Arguments, "workload"))
	if namespace == "" || workload == "" {
		err := fmt.Errorf("namespace and workload are required")
		return errorResult(err), err
	}
	severity := strings.ToUpper(strings.TrimSpace(argString(req.Arguments, "severity")))
	if severity == "" {
		severity = "WARNING"
	}
	// Reject arbitrary severity values — only the Cloud Logging severity set
	// is allowed. Without this, the value flows into a "severity>=%s" clause
	// in the backend filter and a malicious caller could splice in arbitrary
	// AND/OR predicates.
	if _, err := severitiesAtOrAbove(severity); err != nil {
		return errorResult(err), err
	}
	resourceType := strings.TrimSpace(argString(req.Arguments, "resourceType"))
	window := parseDuration(argString(req.Arguments, "duration"), 30*time.Minute)
	limit := argInt(req.Arguments, "limit", 100)
	res, err := backend.Logs().WorkloadEntries(ctx, project, resourceType, namespace, workload, severity, window, limit)
	if err != nil {
		return errorResult(err), err
	}
	resources := []string{fmt.Sprintf("%s/%s", namespace, workload)}
	return mcp.ToolResult{
		Data: map[string]any{
			"backend":      backend.Name(),
			"project":      res.Project,
			"namespace":    namespace,
			"workload":     workload,
			"severity":     severity,
			"resourceType": resourceTypeOrDefault(resourceType),
			"window":       window.String(),
			"filter":       res.Filter,
			"entries":      res.Entries,
			"count":        len(res.Entries),
		},
		Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: resources},
	}, nil
}

func (t *Toolset) handleErrorTimeline(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	namespace := strings.TrimSpace(argString(req.Arguments, "namespace"))
	if namespace == "" {
		err := fmt.Errorf("namespace is required")
		return errorResult(err), err
	}
	workload := strings.TrimSpace(argString(req.Arguments, "workload"))
	severity := strings.ToUpper(strings.TrimSpace(argString(req.Arguments, "severity")))
	if severity == "" {
		severity = "ERROR"
	}
	resourceType := strings.TrimSpace(argString(req.Arguments, "resourceType"))
	if resourceType == "" {
		resourceType = "k8s_container"
	}
	window := parseDuration(argString(req.Arguments, "duration"), time.Hour)
	bucket := parseDuration(argString(req.Arguments, "bucketSize"), 5*time.Minute)
	if bucket <= 0 {
		bucket = 5 * time.Minute
	}
	if bucket > window {
		err := fmt.Errorf("bucketSize (%s) must be <= duration (%s)", bucket, window)
		return errorResult(err), err
	}
	severities, err := severitiesAtOrAbove(severity)
	if err != nil {
		return errorResult(err), err
	}
	res, err := backend.Logs().BucketedErrorCounts(ctx, project, resourceType, namespace, workload, severities, window, bucket)
	if err != nil {
		return errorResult(err), err
	}
	resources := []string{}
	if workload != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", namespace, workload))
	}
	return mcp.ToolResult{
		Data: map[string]any{
			"backend":        backend.Name(),
			"project":        res.Project,
			"namespace":      namespace,
			"workload":       workload,
			"severity":       severity,
			"resourceType":   resourceType,
			"window":         window.String(),
			"bucketSize":     bucket.String(),
			"query":          res.Query,
			"buckets":        res.Buckets,
			"bucketCount":    len(res.Buckets),
			"totalCount":     res.TotalCount,
			"severityCounts": res.SeverityCounts,
		},
		Metadata: mcp.ToolMetadata{Namespaces: nonEmptyList(namespace), Resources: resources},
	}, nil
}

func (t *Toolset) handleCorrelatedWithBundle(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	namespace := strings.TrimSpace(argString(req.Arguments, "namespace"))
	workload := strings.TrimSpace(argString(req.Arguments, "workload"))
	startRaw := strings.TrimSpace(argString(req.Arguments, "startTime"))
	endRaw := strings.TrimSpace(argString(req.Arguments, "endTime"))

	if bundle, ok := req.Arguments["bundle"].(map[string]any); ok {
		if namespace == "" {
			namespace = strings.TrimSpace(argString(bundle, "namespace"))
		}
		if startRaw == "" {
			startRaw = strings.TrimSpace(argString(bundle, "startTime"))
		}
		if endRaw == "" {
			endRaw = strings.TrimSpace(argString(bundle, "endTime"))
		}
		if startRaw == "" || endRaw == "" {
			s, e := extractBundleWindow(bundle)
			if startRaw == "" && !s.IsZero() {
				startRaw = s.Format(time.RFC3339)
			}
			if endRaw == "" && !e.IsZero() {
				endRaw = e.Format(time.RFC3339)
			}
		}
	}

	severity := strings.ToUpper(strings.TrimSpace(argString(req.Arguments, "severity")))
	if severity == "" {
		severity = "WARNING"
	}
	// Same allowlist rationale as handleLogsWorkload — severity flows into
	// the backend filter, so an unvalidated value would allow filter splicing.
	if _, err := severitiesAtOrAbove(severity); err != nil {
		return errorResult(err), err
	}
	resourceType := strings.TrimSpace(argString(req.Arguments, "resourceType"))
	limit := argInt(req.Arguments, "limit", 200)
	fallback := parseDuration(argString(req.Arguments, "duration"), 30*time.Minute)
	startT, endT, err := resolveTimeRange(startRaw, endRaw, fallback)
	if err != nil {
		return errorResult(err), err
	}
	if namespace == "" {
		err := fmt.Errorf("namespace is required (provide directly or via bundle)")
		return errorResult(err), err
	}
	res, err := backend.Logs().EntriesInWindow(ctx, project, resourceType, namespace, workload, severity, startT, endT, limit)
	if err != nil {
		return errorResult(err), err
	}
	resources := []string{}
	if workload != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", namespace, workload))
	}
	return mcp.ToolResult{
		Data: map[string]any{
			"backend":      backend.Name(),
			"project":      res.Project,
			"namespace":    namespace,
			"workload":     workload,
			"severity":     severity,
			"resourceType": resourceTypeOrDefault(resourceType),
			"startTime":    startT.UTC().Format(time.RFC3339),
			"endTime":      endT.UTC().Format(time.RFC3339),
			"filter":       res.Filter,
			"entries":      res.Entries,
			"count":        len(res.Entries),
		},
		Metadata: mcp.ToolMetadata{Namespaces: nonEmptyList(namespace), Resources: resources},
	}, nil
}

// --- helpers shared across handlers ---

func severitiesAtOrAbove(min string) ([]string, error) {
	order := []string{"DEFAULT", "DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY"}
	idx := -1
	for i, s := range order {
		if s == min {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("invalid severity %q", min)
	}
	return order[idx:], nil
}

func extractBundleWindow(bundle map[string]any) (time.Time, time.Time) {
	var earliest, latest time.Time
	observe := func(t time.Time) {
		if t.IsZero() {
			return
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		if latest.IsZero() || t.After(latest) {
			latest = t
		}
	}
	scanTime(bundle["startTime"], observe)
	scanTime(bundle["endTime"], observe)
	sections, ok := bundle["sections"].(map[string]any)
	if !ok {
		return earliest, latest
	}
	if events, _ := sections["eventsTimeline"].(map[string]any); events != nil {
		scanList(events["timeline"], "time", observe)
	}
	if helm, _ := sections["helmReleases"].(map[string]any); helm != nil {
		scanList(helm["releases"], "updated", observe)
	}
	return earliest, latest
}

func scanList(value any, timeKey string, observe func(time.Time)) {
	switch list := value.(type) {
	case []any:
		for _, raw := range list {
			if item, ok := raw.(map[string]any); ok {
				if t, err := parseRFC3339(argString(item, timeKey)); err == nil {
					observe(t)
				}
			}
		}
	case []map[string]any:
		for _, item := range list {
			if t, err := parseRFC3339(argString(item, timeKey)); err == nil {
				observe(t)
			}
		}
	}
}

func scanTime(value any, observe func(time.Time)) {
	if raw := strings.TrimSpace(toString(value)); raw != "" {
		if t, err := parseRFC3339(raw); err == nil {
			observe(t)
		}
	}
}

func resolveTimeRange(startRaw, endRaw string, fallback time.Duration) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	var startT, endT time.Time
	var err error
	if endRaw != "" {
		endT, err = parseRFC3339(endRaw)
		if err != nil {
			return startT, endT, fmt.Errorf("invalid endTime: %w", err)
		}
	} else {
		endT = now
	}
	if startRaw != "" {
		startT, err = parseRFC3339(startRaw)
		if err != nil {
			return startT, endT, fmt.Errorf("invalid startTime: %w", err)
		}
	} else {
		startT = endT.Add(-fallback)
	}
	if !startT.Before(endT) {
		return startT, endT, fmt.Errorf("startTime must be before endTime")
	}
	return startT, endT, nil
}

func nonEmptyList(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func schemaLogsQuery() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string"},
			"filter":    map[string]any{"type": "string", "description": "Backend-native log filter (Cloud Logging filter for GCP)."},
			"duration":  map[string]any{"type": "string", "description": "Window duration. Default 30m."},
			"limit":     map[string]any{"type": "integer", "description": "Max entries (default 100, max 500)."},
		},
		"required":             []string{"filter"},
		"additionalProperties": true,
	}
}

func schemaLogsWorkload() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId":    map[string]any{"type": "string"},
			"namespace":    map[string]any{"type": "string"},
			"workload":     map[string]any{"type": "string"},
			"severity":     map[string]any{"type": "string", "description": "Minimum severity. Default WARNING."},
			"resourceType": map[string]any{"type": "string", "description": "Monitored resource type (default k8s_container). Use e.g. generic_node for self-managed clusters."},
			"duration":     map[string]any{"type": "string", "description": "Window duration. Default 30m."},
			"limit":        map[string]any{"type": "integer", "description": "Max entries (default 100, max 500)."},
		},
		"required":             []string{"namespace", "workload"},
		"additionalProperties": true,
	}
}

func schemaErrorTimeline() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId":    map[string]any{"type": "string"},
			"namespace":    map[string]any{"type": "string", "description": "Kubernetes namespace (required)."},
			"workload":     map[string]any{"type": "string", "description": "Optional workload narrowing."},
			"severity":     map[string]any{"type": "string", "description": "Minimum severity (default ERROR)."},
			"duration":     map[string]any{"type": "string", "description": "Window duration. Default 1h."},
			"bucketSize":   map[string]any{"type": "string", "description": "Bucket size. Default 5m."},
			"resourceType": map[string]any{"type": "string", "description": "Monitored resource type (default k8s_container)."},
		},
		"required":             []string{"namespace"},
		"additionalProperties": true,
	}
}

func schemaCorrelatedWithBundle() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId":    map[string]any{"type": "string"},
			"bundle":       map[string]any{"type": "object", "description": "Optional rootcause incident bundle."},
			"namespace":    map[string]any{"type": "string", "description": "Required unless derivable from bundle."},
			"workload":     map[string]any{"type": "string"},
			"resourceType": map[string]any{"type": "string", "description": "Monitored resource type (default k8s_container)."},
			"startTime":    map[string]any{"type": "string", "description": "RFC3339 start."},
			"endTime":      map[string]any{"type": "string", "description": "RFC3339 end."},
			"severity":     map[string]any{"type": "string", "description": "Default WARNING."},
			"duration":     map[string]any{"type": "string", "description": "Fallback window when start/end missing."},
			"limit":        map[string]any{"type": "integer", "description": "Max entries (default 200, max 500)."},
		},
		"additionalProperties": true,
	}
}
