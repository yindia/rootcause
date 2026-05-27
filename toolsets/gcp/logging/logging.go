package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/logging"

	"rootcause/internal/mcp"
	"rootcause/toolsets/gcp/monitoring"
)

type Service struct {
	ctx       mcp.ToolContext
	toolsetID string
	api       API
	// metricsAPI is used by gcp.logs.error_timeline to compute accurate
	// bucketed counts from the Cloud Monitoring `log_entry_count` metric
	// rather than paginating Cloud Logging entries (which is bounded).
	metricsAPI monitoring.API
}

func ToolSpecs(ctx mcp.ToolContext, toolsetID string, api API, metricsAPI monitoring.API) []mcp.ToolSpec {
	svc := &Service{ctx: ctx, toolsetID: toolsetID, api: api, metricsAPI: metricsAPI}
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
			Description: "Bucketed error/warning counts over a time window for a k8s_container workload, computed from the Cloud Monitoring `logging.googleapis.com/log_entry_count` metric. Accurate (not pagination-bounded) and includes per-severity breakdown per bucket.",
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

	finalFilter := withinWindow(filter, window)
	entries, usedProject, err := s.api.FetchEntries(ctx, project, finalFilter, limit)
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

	filter := workloadFilter(namespace, workload, severity, window)
	entries, usedProject, err := s.api.FetchEntries(ctx, project, filter, limit)
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
	if s.metricsAPI == nil {
		err := fmt.Errorf("metrics API is unavailable; error_timeline requires the gcp.metrics service to be initialized")
		return errorResult(err), err
	}
	project := toString(req.Arguments["projectId"])
	namespace := strings.TrimSpace(toString(req.Arguments["namespace"]))
	workload := strings.TrimSpace(toString(req.Arguments["workload"]))
	severity := strings.ToUpper(strings.TrimSpace(toString(req.Arguments["severity"])))
	if severity == "" {
		severity = "ERROR"
	}
	if namespace == "" {
		err := fmt.Errorf("namespace is required")
		return errorResult(err), err
	}
	resourceType := strings.TrimSpace(toString(req.Arguments["resourceType"]))
	if resourceType == "" {
		resourceType = "k8s_container"
	}
	window := parseDuration(toString(req.Arguments["duration"]), time.Hour)
	bucketSize := parseDuration(toString(req.Arguments["bucketSize"]), 5*time.Minute)
	if bucketSize <= 0 {
		bucketSize = 5 * time.Minute
	}

	severities, err := severitiesAtOrAbove(severity)
	if err != nil {
		return errorResult(err), err
	}

	mql := errorTimelineMQL(resourceType, namespace, workload, severities, window, bucketSize)
	series, usedProject, err := s.metricsAPI.RunMQL(ctx, project, mql)
	if err != nil {
		return errorResult(err), err
	}

	now := time.Now().UTC()
	end := now.Truncate(bucketSize).Add(bucketSize)
	start := end.Add(-window).Truncate(bucketSize)
	buckets, totalCount, severityCounts := bucketizeFromTimeSeries(series, start, end, bucketSize)

	resources := []string{}
	if workload != "" {
		resources = append(resources, fmt.Sprintf("%s/%s", namespace, workload))
	}
	out := map[string]any{
		"project":        usedProject,
		"namespace":      namespace,
		"workload":       workload,
		"severity":       severity,
		"window":         window.String(),
		"bucketSize":     bucketSize.String(),
		"resourceType":   resourceType,
		"source":         "logging.googleapis.com/log_entry_count",
		"mql":            mql,
		"buckets":        buckets,
		"bucketCount":    len(buckets),
		"totalCount":     totalCount,
		"severityCounts": severityCounts,
	}
	return mcp.ToolResult{
		Data:     out,
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
	entries, usedProject, err := s.api.FetchEntries(ctx, project, filter, limit)
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

// EncodeEntry is exported so the SDK adapter can reuse the flattening logic.
func EncodeEntry(entry *logging.Entry) map[string]any {
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
	scanTimes(bundle["startTime"], observe)
	scanTimes(bundle["endTime"], observe)
	sections, ok := bundle["sections"].(map[string]any)
	if !ok {
		return earliest, latest
	}
	if events, _ := sections["eventsTimeline"].(map[string]any); events != nil {
		scanTimelineEntries(events["timeline"], "time", observe)
	}
	if helm, _ := sections["helmReleases"].(map[string]any); helm != nil {
		scanTimelineEntries(helm["releases"], "updated", observe)
	}
	return earliest, latest
}

func scanTimelineEntries(value any, timeKey string, observe func(time.Time)) {
	switch list := value.(type) {
	case []any:
		for _, raw := range list {
			if item, ok := raw.(map[string]any); ok {
				if t, err := parseTime(toString(item[timeKey])); err == nil {
					observe(t)
				}
			}
		}
	case []map[string]any:
		for _, item := range list {
			if t, err := parseTime(toString(item[timeKey])); err == nil {
				observe(t)
			}
		}
	}
}

func scanTimes(value any, observe func(time.Time)) {
	if raw := strings.TrimSpace(toString(value)); raw != "" {
		if t, err := parseTime(raw); err == nil {
			observe(t)
		}
	}
}

// errorTimelineMQL builds the MQL query that produces per-bucket, per-severity
// log entry counts from the Cloud Monitoring `log_entry_count` metric.
// resourceType selects the monitored resource (default k8s_container); pass e.g.
// generic_node for self-managed clusters routing logs via the Ops Agent.
func errorTimelineMQL(resourceType, namespace, workload string, severities []string, window, bucketSize time.Duration) string {
	filterParts := []string{
		fmt.Sprintf("resource.namespace_name == '%s'", escape(namespace)),
	}
	if workload != "" {
		filterParts = append(filterParts, fmt.Sprintf("(resource.pod_name =~ '%s-.*')", escape(workload)))
	}
	severityClauses := make([]string, 0, len(severities))
	for _, s := range severities {
		severityClauses = append(severityClauses, fmt.Sprintf("metric.severity == '%s'", s))
	}
	severityFilter := strings.Join(severityClauses, " || ")

	return fmt.Sprintf(
		"fetch %s::logging.googleapis.com/log_entry_count | filter %s | filter %s | align delta(%s) | every %s | group_by [metric.severity], sum(val()) | within %s",
		mqlResourceLiteral(resourceType),
		strings.Join(filterParts, " && "),
		severityFilter,
		monitoring.DurationLiteral(bucketSize),
		monitoring.DurationLiteral(bucketSize),
		monitoring.DurationLiteral(window),
	)
}

// mqlResourceLiteral defends against MQL injection via the resourceType arg by
// allowing only the GCP monitored-resource identifier charset.
func mqlResourceLiteral(resourceType string) string {
	rt := strings.TrimSpace(resourceType)
	if rt == "" {
		return "k8s_container"
	}
	for _, r := range rt {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' {
			return "k8s_container"
		}
	}
	return rt
}

// severitiesAtOrAbove returns the ordered list of severities at or above min.
func severitiesAtOrAbove(min string) ([]string, error) {
	order := []string{"DEFAULT", "DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY"}
	min = strings.ToUpper(strings.TrimSpace(min))
	idx := -1
	for i, s := range order {
		if s == min {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("invalid severity %q (expected one of %s)", min, strings.Join(order, "/"))
	}
	return order[idx:], nil
}

// bucketizeFromTimeSeries flattens MQL time-series output into per-bucket counts
// keyed by RFC3339 bucket start. Each series carries a metric.severity label
// in labelValues[0] (single label group_by) which we use for the breakdown.
func bucketizeFromTimeSeries(series []map[string]any, start, end time.Time, bucketSize time.Duration) ([]map[string]any, int, map[string]int) {
	bucketCount := int(end.Sub(start) / bucketSize)
	if bucketCount <= 0 {
		return nil, 0, map[string]int{}
	}
	counts := make([]int, bucketCount)
	severityByBucket := make([]map[string]int, bucketCount)
	for i := range severityByBucket {
		severityByBucket[i] = map[string]int{}
	}
	severityTotals := map[string]int{}
	total := 0

	for _, ts := range series {
		severity := "DEFAULT"
		if labels, ok := ts["labelValues"].([]string); ok && len(labels) > 0 {
			severity = labels[0]
		} else if labels, ok := ts["labelValues"].([]any); ok && len(labels) > 0 {
			if s, ok := labels[0].(string); ok {
				severity = s
			}
		}
		points, _ := ts["points"].([]map[string]any)
		if points == nil {
			if generic, ok := ts["points"].([]any); ok {
				for _, p := range generic {
					if m, ok := p.(map[string]any); ok {
						points = append(points, m)
					}
				}
			}
		}
		for _, p := range points {
			pt, err := parseTime(toString(p["start"]))
			if err != nil {
				if pt2, err2 := parseTime(toString(p["end"])); err2 == nil {
					pt = pt2.Add(-bucketSize)
				} else {
					continue
				}
			}
			if pt.Before(start) || !pt.Before(end) {
				continue
			}
			idx := int(pt.Sub(start) / bucketSize)
			if idx < 0 || idx >= bucketCount {
				continue
			}
			count := int(numericValue(p["value"]))
			counts[idx] += count
			severityByBucket[idx][severity] += count
			severityTotals[severity] += count
			total += count
		}
	}

	out := make([]map[string]any, 0, bucketCount)
	for i := range bucketCount {
		bucketStart := start.Add(time.Duration(i) * bucketSize)
		out = append(out, map[string]any{
			"bucketStart":       bucketStart.Format(time.RFC3339),
			"bucketEnd":         bucketStart.Add(bucketSize).Format(time.RFC3339),
			"count":             counts[i],
			"severityBreakdown": severityByBucket[i],
		})
	}
	return out, total, severityTotals
}

func numericValue(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	}
	return 0
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
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT or GCP_PROJECT env. Observability project is independent of the cluster (EKS/AKS can also ship to GCP), so set it explicitly."},
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
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT or GCP_PROJECT env. Observability project is independent of the cluster (EKS/AKS can also ship to GCP), so set it explicitly."},
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
			"projectId":  map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT or GCP_PROJECT env. Observability project is independent of the cluster (EKS/AKS can also ship to GCP), so set it explicitly."},
			"namespace":  map[string]any{"type": "string", "description": "Kubernetes namespace (required)."},
			"workload":   map[string]any{"type": "string", "description": "Optional workload name to narrow to a single deployment/statefulset."},
			"severity":     map[string]any{"type": "string", "description": "Minimum severity (default ERROR). Counts include this severity and all above."},
			"duration":     map[string]any{"type": "string", "description": "Window duration. Default 1h."},
			"bucketSize":   map[string]any{"type": "string", "description": "Bucket size (e.g. '1m', '5m'). Default 5m."},
			"resourceType": map[string]any{"type": "string", "description": "Monitored resource type (default k8s_container). Use e.g. generic_node for self-managed clusters routing logs via the Ops Agent."},
		},
		"required":             []string{"namespace"},
		"additionalProperties": true,
	}
}

func schemaCorrelatedWithBundle() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT or GCP_PROJECT env. Observability project is independent of the cluster (EKS/AKS can also ship to GCP), so set it explicitly."},
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
