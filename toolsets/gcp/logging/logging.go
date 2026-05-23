package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/iterator"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx       mcp.ToolContext
	toolsetID string
	logClient func(context.Context, string) (*logadmin.Client, string, error)
}

func ToolSpecs(ctx mcp.ToolContext, toolsetID string, logClient func(context.Context, string) (*logadmin.Client, string, error)) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, toolsetID: toolsetID, logClient: logClient}
	return []mcp.ToolSpec{
		{
			Name:        "gcp.logs.query",
			Description: "Run a raw Cloud Logging filter and return matching log entries.",
			ToolsetID:   toolsetID,
			InputSchema: schemaQuery(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleQuery,
		},
		{
			Name:        "gcp.logs.workload",
			Description: "Fetch recent errors and warnings for a Kubernetes workload over a time window.",
			ToolsetID:   toolsetID,
			InputSchema: schemaWorkload(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleWorkload,
		},
		{
			Name:        "gcp.logs.error_timeline",
			Description: "Bucketed error/warning counts over a time window for a workload (or raw filter). Useful for spotting an inflection point.",
			ToolsetID:   toolsetID,
			InputSchema: schemaErrorTimeline(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleErrorTimeline,
		},
		{
			Name:        "gcp.logs.correlated_with_bundle",
			Description: "Pull log entries matching a rootcause incident bundle's event window. Accepts a bundle object or explicit namespace+startTime+endTime.",
			ToolsetID:   toolsetID,
			InputSchema: schemaCorrelatedWithBundle(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleCorrelatedWithBundle,
		},
	}
}

func (s *Service) handleQuery(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	filter := strings.TrimSpace(toString(req.Arguments["filter"]))
	if filter == "" {
		err := fmt.Errorf("filter is required")
		return errorResult(err), err
	}
	window := parseDuration(toString(req.Arguments["duration"]), 30*time.Minute)
	limit := toInt(req.Arguments["limit"], 100)

	client, usedProject, err := s.logClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	finalFilter := withinWindow(filter, window)
	entries, err := fetchEntries(ctx, client, finalFilter, limit)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"project": usedProject,
		"filter":  finalFilter,
		"entries": entries,
		"count":   len(entries),
	}}, nil
}

func (s *Service) handleWorkload(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	namespace := strings.TrimSpace(toString(req.Arguments["namespace"]))
	workload := strings.TrimSpace(toString(req.Arguments["workload"]))
	if namespace == "" || workload == "" {
		err := fmt.Errorf("namespace and workload are required")
		return errorResult(err), err
	}
	severity := strings.ToUpper(strings.TrimSpace(toString(req.Arguments["severity"])))
	if severity == "" {
		severity = "WARNING"
	}
	window := parseDuration(toString(req.Arguments["duration"]), 30*time.Minute)
	limit := toInt(req.Arguments["limit"], 100)

	client, usedProject, err := s.logClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	filter := workloadFilter(namespace, workload, severity, window)
	entries, err := fetchEntries(ctx, client, filter, limit)
	if err != nil {
		return errorResult(err), err
	}
	resources := []string{fmt.Sprintf("%s/%s", namespace, workload)}
	return mcp.ToolResult{
		Data: map[string]any{
			"project":   usedProject,
			"namespace": namespace,
			"workload":  workload,
			"severity":  severity,
			"window":    window.String(),
			"filter":    filter,
			"entries":   entries,
			"count":     len(entries),
		},
		Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: resources},
	}, nil
}

func (s *Service) handleErrorTimeline(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	namespace := strings.TrimSpace(toString(req.Arguments["namespace"]))
	workload := strings.TrimSpace(toString(req.Arguments["workload"]))
	rawFilter := strings.TrimSpace(toString(req.Arguments["filter"]))
	severity := strings.ToUpper(strings.TrimSpace(toString(req.Arguments["severity"])))
	if severity == "" {
		severity = "ERROR"
	}
	window := parseDuration(toString(req.Arguments["duration"]), time.Hour)
	bucketSize := parseDuration(toString(req.Arguments["bucketSize"]), 5*time.Minute)
	if bucketSize <= 0 {
		bucketSize = 5 * time.Minute
	}
	scanLimit := toInt(req.Arguments["scanLimit"], 1000)
	if scanLimit <= 0 || scanLimit > 5000 {
		scanLimit = 1000
	}

	var filter string
	switch {
	case rawFilter != "":
		filter = withinWindow(rawFilter, window)
	case namespace != "" && workload != "":
		filter = workloadFilter(namespace, workload, severity, window)
	case namespace != "":
		base := fmt.Sprintf(
			`resource.type="k8s_container" AND resource.labels.namespace_name="%s" AND severity>=%s`,
			escape(namespace), severity,
		)
		filter = withinWindow(base, window)
	default:
		err := fmt.Errorf("provide either filter, or namespace (optionally with workload)")
		return errorResult(err), err
	}

	client, usedProject, err := s.logClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	entries, err := fetchEntries(ctx, client, filter, scanLimit)
	if err != nil {
		return errorResult(err), err
	}

	now := time.Now().UTC()
	end := now.Truncate(bucketSize).Add(bucketSize)
	start := end.Add(-window).Truncate(bucketSize)
	buckets := bucketize(entries, start, end, bucketSize)
	severityCounts := tallySeverity(entries)
	resources := []string{}
	if namespace != "" && workload != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", namespace, workload))
	}

	return mcp.ToolResult{
		Data: map[string]any{
			"project":        usedProject,
			"namespace":      namespace,
			"workload":       workload,
			"severity":       severity,
			"window":         window.String(),
			"bucketSize":     bucketSize.String(),
			"filter":         filter,
			"buckets":        buckets,
			"bucketCount":    len(buckets),
			"totalCount":     len(entries),
			"severityCounts": severityCounts,
			"truncated":      len(entries) >= scanLimit,
		},
		Metadata: mcp.ToolMetadata{Namespaces: nonEmptyList(namespace), Resources: resources},
	}, nil
}

func (s *Service) handleCorrelatedWithBundle(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	namespace := strings.TrimSpace(toString(req.Arguments["namespace"]))
	workload := strings.TrimSpace(toString(req.Arguments["workload"]))
	startRaw := strings.TrimSpace(toString(req.Arguments["startTime"]))
	endRaw := strings.TrimSpace(toString(req.Arguments["endTime"]))

	if bundle, ok := req.Arguments["bundle"].(map[string]any); ok {
		if namespace == "" {
			namespace = strings.TrimSpace(toString(bundle["namespace"]))
		}
		if startRaw == "" {
			startRaw = strings.TrimSpace(toString(bundle["startTime"]))
		}
		if endRaw == "" {
			endRaw = strings.TrimSpace(toString(bundle["endTime"]))
		}
		if (startRaw == "" || endRaw == "") {
			s, e := extractBundleWindow(bundle)
			if startRaw == "" && !s.IsZero() {
				startRaw = s.Format(time.RFC3339)
			}
			if endRaw == "" && !e.IsZero() {
				endRaw = e.Format(time.RFC3339)
			}
		}
	}

	severity := strings.ToUpper(strings.TrimSpace(toString(req.Arguments["severity"])))
	if severity == "" {
		severity = "WARNING"
	}
	limit := toInt(req.Arguments["limit"], 200)

	startT, endT, err := resolveTimeRange(startRaw, endRaw, parseDuration(toString(req.Arguments["duration"]), 30*time.Minute))
	if err != nil {
		return errorResult(err), err
	}
	if namespace == "" {
		err := fmt.Errorf("namespace is required (provide directly or via bundle)")
		return errorResult(err), err
	}

	filter := bundleWindowFilter(namespace, workload, severity, startT, endT)
	client, usedProject, err := s.logClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	entries, err := fetchEntries(ctx, client, filter, limit)
	if err != nil {
		return errorResult(err), err
	}
	resources := []string{}
	if workload != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", namespace, workload))
	}
	return mcp.ToolResult{
		Data: map[string]any{
			"project":   usedProject,
			"namespace": namespace,
			"workload":  workload,
			"severity":  severity,
			"startTime": startT.UTC().Format(time.RFC3339),
			"endTime":   endT.UTC().Format(time.RFC3339),
			"filter":    filter,
			"entries":   entries,
			"count":     len(entries),
		},
		Metadata: mcp.ToolMetadata{Namespaces: nonEmptyList(namespace), Resources: resources},
	}, nil
}

func fetchEntries(ctx context.Context, client *logadmin.Client, filter string, limit int) ([]map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("logging client is nil")
	}
	if limit <= 0 || limit > 5000 {
		limit = 100
	}
	it := client.Entries(ctx, logadmin.Filter(filter), logadmin.NewestFirst())
	out := make([]map[string]any, 0, limit)
	for len(out) < limit {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, err
		}
		out = append(out, encodeEntry(entry))
	}
	return out, nil
}

func encodeEntry(entry *logging.Entry) map[string]any {
	if entry == nil {
		return nil
	}
	out := map[string]any{
		"timestamp": entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"severity":  entry.Severity.String(),
		"logName":   entry.LogName,
		"payload":   normalizePayload(entry.Payload),
	}
	if entry.InsertID != "" {
		out["insertId"] = entry.InsertID
	}
	if entry.Trace != "" {
		out["trace"] = entry.Trace
	}
	if entry.SpanID != "" {
		out["spanId"] = entry.SpanID
	}
	if len(entry.Labels) > 0 {
		out["labels"] = entry.Labels
	}
	if entry.Resource != nil {
		out["resource"] = map[string]any{
			"type":   entry.Resource.Type,
			"labels": entry.Resource.Labels,
		}
	}
	if entry.HTTPRequest != nil && entry.HTTPRequest.Request != nil {
		out["httpRequest"] = map[string]any{
			"method": entry.HTTPRequest.Request.Method,
			"url":    entry.HTTPRequest.Request.URL.String(),
			"status": entry.HTTPRequest.Status,
		}
	}
	return out
}

func normalizePayload(payload any) any {
	switch v := payload.(type) {
	case nil:
		return nil
	case string:
		return v
	case map[string]any:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func workloadFilter(namespace, workload, severity string, window time.Duration) string {
	base := fmt.Sprintf(
		`resource.type="k8s_container" AND resource.labels.namespace_name="%s" AND resource.labels.pod_name:"%s-"`,
		escape(namespace), escape(workload),
	)
	if severity != "" {
		base += fmt.Sprintf(` AND severity>=%s`, severity)
	}
	return withinWindow(base, window)
}

func bundleWindowFilter(namespace, workload, severity string, start, end time.Time) string {
	parts := []string{
		`resource.type="k8s_container"`,
		fmt.Sprintf(`resource.labels.namespace_name="%s"`, escape(namespace)),
	}
	if workload != "" {
		parts = append(parts, fmt.Sprintf(`resource.labels.pod_name:"%s-"`, escape(workload)))
	}
	if severity != "" {
		parts = append(parts, fmt.Sprintf(`severity>=%s`, severity))
	}
	parts = append(parts,
		fmt.Sprintf(`timestamp>="%s"`, start.UTC().Format(time.RFC3339)),
		fmt.Sprintf(`timestamp<="%s"`, end.UTC().Format(time.RFC3339)),
	)
	return strings.Join(parts, " AND ")
}

func withinWindow(filter string, window time.Duration) string {
	if window <= 0 {
		window = 30 * time.Minute
	}
	since := time.Now().UTC().Add(-window).Format(time.RFC3339)
	return fmt.Sprintf(`(%s) AND timestamp>="%s"`, filter, since)
}

func resolveTimeRange(startRaw, endRaw string, fallback time.Duration) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	var startT, endT time.Time
	var err error
	if endRaw != "" {
		endT, err = parseTime(endRaw)
		if err != nil {
			return startT, endT, fmt.Errorf("invalid endTime: %w", err)
		}
	} else {
		endT = now
	}
	if startRaw != "" {
		startT, err = parseTime(startRaw)
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

func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 timestamp, got %q", raw)
}

func extractBundleWindow(bundle map[string]any) (time.Time, time.Time) {
	var earliest, latest time.Time
	sections, ok := bundle["sections"].(map[string]any)
	if !ok {
		return earliest, latest
	}
	events, _ := sections["eventsTimeline"].(map[string]any)
	if events == nil {
		return earliest, latest
	}
	timeline, _ := events["timeline"].([]any)
	if len(timeline) == 0 {
		if generic, okGen := events["timeline"].([]map[string]any); okGen {
			for _, item := range generic {
				if t, err := parseTime(toString(item["time"])); err == nil {
					if earliest.IsZero() || t.Before(earliest) {
						earliest = t
					}
					if latest.IsZero() || t.After(latest) {
						latest = t
					}
				}
			}
		}
		return earliest, latest
	}
	for _, raw := range timeline {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t, err := parseTime(toString(item["time"]))
		if err != nil {
			continue
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		if latest.IsZero() || t.After(latest) {
			latest = t
		}
	}
	return earliest, latest
}

func bucketize(entries []map[string]any, start, end time.Time, bucketSize time.Duration) []map[string]any {
	if bucketSize <= 0 || !start.Before(end) {
		return nil
	}
	bucketCount := int(end.Sub(start) / bucketSize)
	if bucketCount <= 0 {
		return nil
	}
	counts := make([]int, bucketCount)
	severityByBucket := make([]map[string]int, bucketCount)
	for i := range severityByBucket {
		severityByBucket[i] = map[string]int{}
	}
	for _, e := range entries {
		t, err := parseTime(toString(e["timestamp"]))
		if err != nil {
			continue
		}
		if t.Before(start) || !t.Before(end) {
			continue
		}
		idx := int(t.Sub(start) / bucketSize)
		if idx < 0 || idx >= bucketCount {
			continue
		}
		counts[idx]++
		sev := strings.ToUpper(strings.TrimSpace(toString(e["severity"])))
		if sev == "" {
			sev = "DEFAULT"
		}
		severityByBucket[idx][sev]++
	}
	out := make([]map[string]any, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		bucketStart := start.Add(time.Duration(i) * bucketSize)
		out = append(out, map[string]any{
			"bucketStart":       bucketStart.Format(time.RFC3339),
			"bucketEnd":         bucketStart.Add(bucketSize).Format(time.RFC3339),
			"count":             counts[i],
			"severityBreakdown": severityByBucket[i],
		})
	}
	return out
}

func tallySeverity(entries []map[string]any) map[string]int {
	out := map[string]int{}
	for _, e := range entries {
		sev := strings.ToUpper(strings.TrimSpace(toString(e["severity"])))
		if sev == "" {
			sev = "DEFAULT"
		}
		out[sev]++
	}
	return out
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

func escape(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return fallback
}

func toInt(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	}
	return fallback
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: mcp.BuildErrorEnvelope(err, nil)}
}

func schemaQuery() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"filter":    map[string]any{"type": "string", "description": "Cloud Logging Query Language filter expression."},
			"duration":  map[string]any{"type": "string", "description": "Window duration (e.g. '15m', '1h'). Default 30m."},
			"limit":     map[string]any{"type": "integer", "description": "Max entries to return (default 100, max 500)."},
		},
		"required":             []string{"filter"},
		"additionalProperties": true,
	}
}

func schemaWorkload() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"namespace": map[string]any{"type": "string", "description": "Kubernetes namespace."},
			"workload":  map[string]any{"type": "string", "description": "Workload name."},
			"severity":  map[string]any{"type": "string", "description": "Minimum severity (DEBUG/INFO/NOTICE/WARNING/ERROR/CRITICAL). Default WARNING."},
			"duration":  map[string]any{"type": "string", "description": "Window duration. Default 30m."},
			"limit":     map[string]any{"type": "integer", "description": "Max entries (default 100, max 500)."},
		},
		"required":             []string{"namespace", "workload"},
		"additionalProperties": true,
	}
}

func schemaErrorTimeline() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId":  map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"namespace":  map[string]any{"type": "string", "description": "Kubernetes namespace. Required unless `filter` is supplied."},
			"workload":   map[string]any{"type": "string", "description": "Optional workload name to narrow to a single deployment/statefulset."},
			"severity":   map[string]any{"type": "string", "description": "Minimum severity (default ERROR)."},
			"duration":   map[string]any{"type": "string", "description": "Window duration. Default 1h."},
			"bucketSize": map[string]any{"type": "string", "description": "Bucket size (e.g. '1m', '5m'). Default 5m."},
			"filter":     map[string]any{"type": "string", "description": "Raw Cloud Logging filter override. Mutually exclusive with namespace/workload."},
			"scanLimit":  map[string]any{"type": "integer", "description": "Max entries to scan into buckets (default 1000, max 5000)."},
		},
		"additionalProperties": true,
	}
}

func schemaCorrelatedWithBundle() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"bundle":    map[string]any{"type": "object", "description": "Optional rootcause incident bundle. Namespace and time window are auto-derived when present."},
			"namespace": map[string]any{"type": "string", "description": "Kubernetes namespace. Required unless derivable from `bundle`."},
			"workload":  map[string]any{"type": "string", "description": "Optional workload name."},
			"startTime": map[string]any{"type": "string", "description": "RFC3339 start of correlation window."},
			"endTime":   map[string]any{"type": "string", "description": "RFC3339 end of correlation window."},
			"severity":  map[string]any{"type": "string", "description": "Minimum severity (default WARNING)."},
			"duration":  map[string]any{"type": "string", "description": "Fallback window when startTime/endTime are missing. Default 30m."},
			"limit":     map[string]any{"type": "integer", "description": "Max entries (default 200, max 500)."},
		},
		"additionalProperties": true,
	}
}
