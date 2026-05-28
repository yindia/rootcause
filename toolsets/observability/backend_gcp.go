package observability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	label "google.golang.org/genproto/googleapis/api/label"

	gcpcfg "rootcause/internal/gcp"
)

// gcpBackend implements Backend on top of GCP Cloud Monitoring + Cloud Logging
// (a.k.a. Stackdriver). Clients are constructed lazily per (service, project)
// pair and cached.
type gcpBackend struct {
	cfgProject string // observability.gcp.project — used when projectId arg is empty
	credsFile  string // resolved credentials file (env > observability.gcp > [gcp])
	clientPool sync.Map
	sf         singleflight.Group
}

// newGCPBackend constructs the GCP backend. cfgProject is the
// observability.gcp.project from config; cfgCreds is the resolved credentials
// path (observability.gcp.credentials_file with [gcp].credentials_file
// fallback).
func newGCPBackend(cfgProject, cfgCreds string) *gcpBackend {
	return &gcpBackend{cfgProject: cfgProject, credsFile: cfgCreds}
}

func (b *gcpBackend) Name() string       { return "gcp" }
func (b *gcpBackend) Metrics() MetricsAPI { return &gcpMetrics{backend: b} }
func (b *gcpBackend) Logs() LogsAPI       { return &gcpLogs{backend: b} }

type clientEntry struct {
	client  any
	project string
}

// loadClient resolves the project, then builds (or retrieves) a cached client
// for (service, project). Same singleflight semantics as the old gcp toolset.
func (b *gcpBackend) loadClient(ctx context.Context, service, projectExplicit string, build func(ctx context.Context, project string, opts []option.ClientOption) (any, error)) (any, string, error) {
	project := gcpcfg.ResolveProjectWithConfig(projectExplicit, b.cfgProject)
	if project == "" {
		return nil, "", errors.New("gcp project id is required: pass projectId, set GOOGLE_CLOUD_PROJECT / GCP_PROJECT, or set observability.gcp.project in config.yaml (observability project is independent of the cluster's control plane — EKS/AKS clusters can also ship to GCP)")
	}
	fullKey := service + "|" + project
	if raw, ok := b.clientPool.Load(fullKey); ok {
		entry := raw.(*clientEntry)
		return entry.client, entry.project, nil
	}
	resolved, err, _ := b.sf.Do(fullKey, func() (any, error) {
		if raw, ok := b.clientPool.Load(fullKey); ok {
			return raw, nil
		}
		opts := []option.ClientOption{}
		if creds := gcpcfg.CredentialsFileWithConfig(b.credsFile); creds != "" {
			opts = append(opts, option.WithCredentialsFile(creds))
		}
		client, err := build(ctx, project, opts)
		if err != nil {
			return nil, err
		}
		entry := &clientEntry{client: client, project: project}
		b.clientPool.Store(fullKey, entry)
		return entry, nil
	})
	if err != nil {
		return nil, "", err
	}
	return resolved.(*clientEntry).client, resolved.(*clientEntry).project, nil
}

func (b *gcpBackend) queryClient(ctx context.Context, project string) (*monitoring.QueryClient, string, error) {
	raw, used, err := b.loadClient(ctx, "monitoring.query", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewQueryClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.QueryClient), used, nil
}

func (b *gcpBackend) metricClient(ctx context.Context, project string) (*monitoring.MetricClient, string, error) {
	raw, used, err := b.loadClient(ctx, "monitoring.metric", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewMetricClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.MetricClient), used, nil
}

func (b *gcpBackend) slmClient(ctx context.Context, project string) (*monitoring.ServiceMonitoringClient, string, error) {
	raw, used, err := b.loadClient(ctx, "monitoring.slm", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewServiceMonitoringClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.ServiceMonitoringClient), used, nil
}

func (b *gcpBackend) logClient(ctx context.Context, project string) (*logadmin.Client, string, error) {
	raw, used, err := b.loadClient(ctx, "logging.admin", project, func(ctx context.Context, project string, opts []option.ClientOption) (any, error) {
		return logadmin.NewClient(ctx, project, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*logadmin.Client), used, nil
}

// gcpMetrics implements MetricsAPI using Cloud Monitoring.
type gcpMetrics struct {
	backend *gcpBackend
}

func (g *gcpMetrics) RawQuery(ctx context.Context, project, mql string) ([]map[string]any, string, error) {
	client, usedProject, err := g.backend.queryClient(ctx, project)
	if err != nil {
		return nil, "", err
	}
	return runMQL(ctx, client, usedProject, mql)
}

func (g *gcpMetrics) WorkloadMetrics(ctx context.Context, project, resourceType, namespace, workload string, window time.Duration) (WorkloadMetricsResult, error) {
	client, usedProject, err := g.backend.queryClient(ctx, project)
	if err != nil {
		return WorkloadMetricsResult{}, err
	}
	rt := mqlResourceLiteral(resourceType)
	cpu, _, cpuErr := runMQL(ctx, client, usedProject, workloadCPUQuery(rt, namespace, workload, window))
	memory, _, memErr := runMQL(ctx, client, usedProject, workloadMemoryQuery(rt, namespace, workload, window))
	restarts, _, restartErr := runMQL(ctx, client, usedProject, workloadRestartQuery(rt, namespace, workload, window))
	errs := map[string]string{}
	if cpuErr != nil {
		errs["cpu"] = cpuErr.Error()
	}
	if memErr != nil {
		errs["memory"] = memErr.Error()
	}
	if restartErr != nil {
		errs["restartCount"] = restartErr.Error()
	}
	if len(errs) == 0 {
		errs = nil
	}
	return WorkloadMetricsResult{
		Project: usedProject, CPU: cpu, Memory: memory, RestartCount: restarts, Errors: errs,
	}, nil
}

func (g *gcpMetrics) ListDescriptors(ctx context.Context, project, filter string, limit int) ([]map[string]any, string, error) {
	client, usedProject, err := g.backend.metricClient(ctx, project)
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	it := client.ListMetricDescriptors(ctx, &monitoringpb.ListMetricDescriptorsRequest{
		Name:   "projects/" + usedProject,
		Filter: filter,
	})
	out := make([]map[string]any, 0, limit)
	for len(out) < limit {
		md, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, usedProject, err
		}
		out = append(out, map[string]any{
			"type":        md.GetType(),
			"displayName": md.GetDisplayName(),
			"description": md.GetDescription(),
			"unit":        md.GetUnit(),
			"metricKind":  md.GetMetricKind().String(),
			"valueType":   md.GetValueType().String(),
			"labels":      encodeLabelDescriptors(md.GetLabels()),
		})
	}
	return out, usedProject, nil
}

func (g *gcpMetrics) ListServices(ctx context.Context, project string, limit int) ([]map[string]any, string, error) {
	client, usedProject, err := g.backend.slmClient(ctx, project)
	if err != nil {
		return nil, "", err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	it := client.ListServices(ctx, &monitoringpb.ListServicesRequest{Parent: "projects/" + usedProject})
	out := make([]map[string]any, 0, limit)
	for len(out) < limit {
		svc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, usedProject, err
		}
		out = append(out, map[string]any{
			"name":        svc.GetName(),
			"id":          serviceIDFromName(svc.GetName()),
			"displayName": svc.GetDisplayName(),
		})
	}
	return out, usedProject, nil
}

func (g *gcpMetrics) ListSLOs(ctx context.Context, serviceName string, limit int) ([]map[string]any, error) {
	project := projectFromServiceName(serviceName)
	client, _, err := g.backend.slmClient(ctx, project)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	it := client.ListServiceLevelObjectives(ctx, &monitoringpb.ListServiceLevelObjectivesRequest{Parent: serviceName})
	out := make([]map[string]any, 0, limit)
	for len(out) < limit {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, err
		}
		out = append(out, encodeSLO(obj))
	}
	return out, nil
}

// gcpLogs implements LogsAPI using Cloud Logging.
type gcpLogs struct {
	backend *gcpBackend
}

func (l *gcpLogs) WorkloadEntries(ctx context.Context, project, resourceType, namespace, workload, severity string, window time.Duration, limit int) (WorkloadEntriesResult, error) {
	if severity == "" {
		severity = "WARNING"
	}
	filter := workloadFilter(filterResourceLiteral(resourceType), namespace, workload, severity, window)
	return l.fetch(ctx, project, filter, limit)
}

func (l *gcpLogs) EntriesInWindow(ctx context.Context, project, resourceType, namespace, workload, severity string, start, end time.Time, limit int) (WorkloadEntriesResult, error) {
	if severity == "" {
		severity = "WARNING"
	}
	filter := bundleWindowFilter(filterResourceLiteral(resourceType), namespace, workload, severity, start, end)
	return l.fetch(ctx, project, filter, limit)
}

func (l *gcpLogs) RawQuery(ctx context.Context, project, filter string, window time.Duration, limit int) (WorkloadEntriesResult, error) {
	final := withinWindow(filter, window)
	return l.fetch(ctx, project, final, limit)
}

func (l *gcpLogs) BucketedErrorCounts(ctx context.Context, project, resourceType, namespace, workload string, severities []string, window, bucket time.Duration) (BucketedCountsResult, error) {
	mql := errorTimelineMQL(resourceType, namespace, workload, severities, window, bucket)
	client, usedProject, err := l.backend.queryClient(ctx, project)
	if err != nil {
		return BucketedCountsResult{}, err
	}
	series, _, err := runMQL(ctx, client, usedProject, mql)
	if err != nil {
		return BucketedCountsResult{}, err
	}
	now := time.Now().UTC()
	end := now.Truncate(bucket).Add(bucket)
	start := end.Add(-window).Truncate(bucket)
	buckets, total, severityCounts := bucketizeFromTimeSeries(series, start, end, bucket)
	return BucketedCountsResult{
		Project: usedProject, Query: mql,
		Buckets: buckets, TotalCount: total, SeverityCounts: severityCounts,
	}, nil
}

func (l *gcpLogs) fetch(ctx context.Context, project, filter string, limit int) (WorkloadEntriesResult, error) {
	client, usedProject, err := l.backend.logClient(ctx, project)
	if err != nil {
		return WorkloadEntriesResult{}, err
	}
	if limit <= 0 || limit > 500 {
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
			return WorkloadEntriesResult{Project: usedProject, Filter: filter, Entries: out}, err
		}
		out = append(out, encodeLogEntry(entry))
	}
	return WorkloadEntriesResult{Project: usedProject, Filter: filter, Entries: out}, nil
}

// --- shared encoding + query builders (moved from toolsets/gcp/...) ---

func runMQL(ctx context.Context, client *monitoring.QueryClient, project, mql string) ([]map[string]any, string, error) {
	if client == nil {
		return nil, project, fmt.Errorf("monitoring query client is nil")
	}
	it := client.QueryTimeSeries(ctx, &monitoringpb.QueryTimeSeriesRequest{
		Name:  "projects/" + project,
		Query: mql,
	})
	out := make([]map[string]any, 0)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, project, err
		}
		out = append(out, encodeTimeSeriesData(resp))
		if len(out) >= 200 {
			break
		}
	}
	return out, project, nil
}

func encodeTimeSeriesData(ts *monitoringpb.TimeSeriesData) map[string]any {
	labels := make([]string, 0, len(ts.LabelValues))
	for _, lv := range ts.LabelValues {
		labels = append(labels, lv.GetStringValue())
	}
	points := make([]map[string]any, 0, len(ts.PointData))
	for _, p := range ts.PointData {
		pt := map[string]any{}
		if p.TimeInterval != nil {
			if p.TimeInterval.StartTime != nil {
				pt["start"] = p.TimeInterval.StartTime.AsTime().Format(time.RFC3339)
			}
			if p.TimeInterval.EndTime != nil {
				pt["end"] = p.TimeInterval.EndTime.AsTime().Format(time.RFC3339)
			}
		}
		if len(p.Values) > 0 {
			pt["value"] = encodeTypedValue(p.Values[0])
		}
		points = append(points, pt)
	}
	return map[string]any{
		"labelValues": labels,
		"points":      points,
		"pointCount":  len(points),
	}
}

func encodeTypedValue(tv *monitoringpb.TypedValue) any {
	switch v := tv.Value.(type) {
	case *monitoringpb.TypedValue_DoubleValue:
		return v.DoubleValue
	case *monitoringpb.TypedValue_Int64Value:
		return v.Int64Value
	case *monitoringpb.TypedValue_BoolValue:
		return v.BoolValue
	case *monitoringpb.TypedValue_StringValue:
		return v.StringValue
	default:
		return nil
	}
}

func encodeLabelDescriptors(labels []*label.LabelDescriptor) []map[string]any {
	out := make([]map[string]any, 0, len(labels))
	for _, l := range labels {
		out = append(out, map[string]any{
			"key":         l.GetKey(),
			"valueType":   l.GetValueType().String(),
			"description": l.GetDescription(),
		})
	}
	return out
}

func encodeSLO(obj *monitoringpb.ServiceLevelObjective) map[string]any {
	out := map[string]any{
		"name":        obj.GetName(),
		"displayName": obj.GetDisplayName(),
		"goal":        obj.GetGoal(),
	}
	if rp := obj.GetRollingPeriod(); rp != nil {
		out["rollingPeriod"] = rp.AsDuration().String()
	}
	if cp := obj.GetCalendarPeriod(); cp != 0 {
		out["calendarPeriod"] = cp.String()
	}
	if sli := obj.GetServiceLevelIndicator(); sli != nil {
		switch sli.Type.(type) {
		case *monitoringpb.ServiceLevelIndicator_BasicSli:
			out["indicatorType"] = "basicSli"
		case *monitoringpb.ServiceLevelIndicator_RequestBased:
			out["indicatorType"] = "requestBased"
		case *monitoringpb.ServiceLevelIndicator_WindowsBased:
			out["indicatorType"] = "windowsBased"
		}
	}
	return out
}

func encodeLogEntry(entry *logging.Entry) map[string]any {
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

func serviceIDFromName(fullName string) string {
	idx := strings.LastIndex(fullName, "/services/")
	if idx < 0 {
		return fullName
	}
	return fullName[idx+len("/services/"):]
}

func projectFromServiceName(name string) string {
	const prefix = "projects/"
	if len(name) <= len(prefix) || name[:len(prefix)] != prefix {
		return ""
	}
	rest := name[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' {
			return rest[:i]
		}
	}
	return rest
}

// --- query builders (Cloud Monitoring MQL + Cloud Logging filters) ---
//
// resourceType is the monitored-resource identifier (k8s_container,
// generic_node, etc.). Callers must pre-sanitise via mqlResourceLiteral /
// filterResourceLiteral so we never inject raw user input into the query.

func workloadCPUQuery(resourceType, namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch %s | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/cpu/core_usage_time' | rate(1m) | within %s",
		resourceType, escapeQuote(namespace), escapeQuote(workload), durationLiteral(window),
	)
}

func workloadMemoryQuery(resourceType, namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch %s | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/memory/used_bytes' | within %s",
		resourceType, escapeQuote(namespace), escapeQuote(workload), durationLiteral(window),
	)
}

func workloadRestartQuery(resourceType, namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch %s | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/restart_count' | within %s",
		resourceType, escapeQuote(namespace), escapeQuote(workload), durationLiteral(window),
	)
}

func errorTimelineMQL(resourceType, namespace, workload string, severities []string, window, bucketSize time.Duration) string {
	filterParts := []string{
		fmt.Sprintf("resource.namespace_name == '%s'", escapeQuote(namespace)),
	}
	if workload != "" {
		filterParts = append(filterParts, fmt.Sprintf("(resource.pod_name =~ '%s-.*')", escapeQuote(workload)))
	}
	severityClauses := make([]string, 0, len(severities))
	for _, s := range severities {
		severityClauses = append(severityClauses, fmt.Sprintf("metric.severity == '%s'", s))
	}
	return fmt.Sprintf(
		"fetch %s::logging.googleapis.com/log_entry_count | filter %s | filter %s | align delta(%s) | every %s | group_by [metric.severity], sum(val()) | within %s",
		mqlResourceLiteral(resourceType),
		strings.Join(filterParts, " && "),
		strings.Join(severityClauses, " || "),
		durationLiteral(bucketSize),
		durationLiteral(bucketSize),
		durationLiteral(window),
	)
}

func mqlResourceLiteral(resourceType string) string {
	return sanitizeResourceType(resourceType)
}

// filterResourceLiteral is the Cloud Logging filter equivalent of
// mqlResourceLiteral — same identifier charset, same fallback behavior.
func filterResourceLiteral(resourceType string) string {
	return sanitizeResourceType(resourceType)
}

// sanitizeResourceType returns resourceType when it matches the GCP
// monitored-resource identifier charset ([A-Za-z0-9_]); otherwise falls back
// to the safe default k8s_container. Used by both the MQL fetch clause and
// Cloud Logging filter resource.type predicate.
func sanitizeResourceType(resourceType string) string {
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

func workloadFilter(resourceType, namespace, workload, severity string, window time.Duration) string {
	base := fmt.Sprintf(
		`resource.type="%s" AND resource.labels.namespace_name="%s" AND resource.labels.pod_name:"%s-"`,
		resourceType, escapeFilter(namespace), escapeFilter(workload),
	)
	if severity != "" {
		base += fmt.Sprintf(` AND severity>=%s`, severity)
	}
	return withinWindow(base, window)
}

func bundleWindowFilter(resourceType, namespace, workload, severity string, start, end time.Time) string {
	parts := []string{
		fmt.Sprintf(`resource.type="%s"`, resourceType),
		fmt.Sprintf(`resource.labels.namespace_name="%s"`, escapeFilter(namespace)),
	}
	if workload != "" {
		parts = append(parts, fmt.Sprintf(`resource.labels.pod_name:"%s-"`, escapeFilter(workload)))
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

func escapeQuote(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func escapeFilter(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

func durationLiteral(d time.Duration) string {
	if d <= 0 {
		return "30m"
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	return fmt.Sprintf("%ds", int(d/time.Second))
}

// bucketizeFromTimeSeries flattens MQL group_by output into per-bucket counts.
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
			pt, err := parseRFC3339(toString(p["start"]))
			if err != nil {
				if pt2, err2 := parseRFC3339(toString(p["end"])); err2 == nil {
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

func parseRFC3339(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("expected RFC3339 timestamp, got %q", raw)
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

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
