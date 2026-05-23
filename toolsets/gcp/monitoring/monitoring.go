package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"google.golang.org/api/iterator"
	label "google.golang.org/genproto/googleapis/api/label"

	"rootcause/internal/mcp"
)

type Service struct {
	ctx          mcp.ToolContext
	toolsetID    string
	queryClient  func(context.Context, string) (*monitoring.QueryClient, string, error)
	metricClient func(context.Context, string) (*monitoring.MetricClient, string, error)
	slmClient    func(context.Context, string) (*monitoring.ServiceMonitoringClient, string, error)
}

func ToolSpecs(
	ctx mcp.ToolContext,
	toolsetID string,
	queryClient func(context.Context, string) (*monitoring.QueryClient, string, error),
	metricClient func(context.Context, string) (*monitoring.MetricClient, string, error),
	slmClient func(context.Context, string) (*monitoring.ServiceMonitoringClient, string, error),
) []mcp.ToolSpec {
	svc := &Service{
		ctx:          ctx,
		toolsetID:    toolsetID,
		queryClient:  queryClient,
		metricClient: metricClient,
		slmClient:    slmClient,
	}
	return []mcp.ToolSpec{
		{
			Name:        "gcp.metrics.query",
			Description: "Run a raw Cloud Monitoring MQL query and return time series.",
			ToolsetID:   toolsetID,
			InputSchema: schemaQuery(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleQuery,
		},
		{
			Name:        "gcp.metrics.workload",
			Description: "Fetch CPU, memory, and restart count metrics for a Kubernetes workload over a time window.",
			ToolsetID:   toolsetID,
			InputSchema: schemaWorkload(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleWorkload,
		},
		{
			Name:        "gcp.metrics.list_descriptors",
			Description: "List Cloud Monitoring metric descriptors for discoverability. Supports a Monitoring filter (e.g. `metric.type=starts_with(\"kubernetes.io/\")`).",
			ToolsetID:   toolsetID,
			InputSchema: schemaListDescriptors(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleListDescriptors,
		},
		{
			Name:        "gcp.metrics.slo_status",
			Description: "List Service Monitoring services and their Service Level Objectives (goal, period, indicator type).",
			ToolsetID:   toolsetID,
			InputSchema: schemaSLOStatus(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     svc.handleSLOStatus,
		},
	}
}

func (s *Service) handleQuery(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	query := strings.TrimSpace(toString(req.Arguments["query"]))
	if query == "" {
		err := fmt.Errorf("query is required")
		return errorResult(err), err
	}
	client, usedProject, err := s.queryClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	series, err := runMQL(ctx, client, usedProject, query)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"project":    usedProject,
		"query":      query,
		"timeSeries": series,
		"count":      len(series),
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
	window := parseDuration(toString(req.Arguments["duration"]), 30*time.Minute)

	client, usedProject, err := s.queryClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	cpu, cpuErr := runMQL(ctx, client, usedProject, workloadCPUQuery(namespace, workload, window))
	memory, memErr := runMQL(ctx, client, usedProject, workloadMemoryQuery(namespace, workload, window))
	restarts, restartErr := runMQL(ctx, client, usedProject, workloadRestartQuery(namespace, workload, window))

	out := map[string]any{
		"project":   usedProject,
		"namespace": namespace,
		"workload":  workload,
		"window":    window.String(),
		"metrics": map[string]any{
			"cpu":          cpu,
			"memory":       memory,
			"restartCount": restarts,
		},
	}
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
	if len(errs) > 0 {
		out["errors"] = errs
	}
	resources := []string{fmt.Sprintf("%s/%s", namespace, workload)}
	return mcp.ToolResult{
		Data:     out,
		Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: resources},
	}, nil
}

func (s *Service) handleListDescriptors(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	filter := strings.TrimSpace(toString(req.Arguments["filter"]))
	limit := toInt(req.Arguments["limit"], 50)
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	client, usedProject, err := s.metricClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	if client == nil {
		err := fmt.Errorf("metric client is nil")
		return errorResult(err), err
	}
	it := client.ListMetricDescriptors(ctx, &monitoringpb.ListMetricDescriptorsRequest{
		Name:   "projects/" + usedProject,
		Filter: filter,
	})
	descriptors := make([]map[string]any, 0, limit)
	for len(descriptors) < limit {
		md, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errorResult(err), err
		}
		descriptors = append(descriptors, map[string]any{
			"type":        md.GetType(),
			"displayName": md.GetDisplayName(),
			"description": md.GetDescription(),
			"unit":        md.GetUnit(),
			"metricKind":  md.GetMetricKind().String(),
			"valueType":   md.GetValueType().String(),
			"labels":      encodeLabelDescriptors(md.GetLabels()),
		})
	}
	return mcp.ToolResult{Data: map[string]any{
		"project":     usedProject,
		"filter":      filter,
		"descriptors": descriptors,
		"count":       len(descriptors),
	}}, nil
}

func (s *Service) handleSLOStatus(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	project := toString(req.Arguments["projectId"])
	serviceFilter := strings.TrimSpace(toString(req.Arguments["serviceId"]))
	limit := toInt(req.Arguments["limit"], 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	client, usedProject, err := s.slmClient(ctx, project)
	if err != nil {
		return errorResult(err), err
	}
	if client == nil {
		err := fmt.Errorf("service monitoring client is nil")
		return errorResult(err), err
	}

	services := make([]map[string]any, 0)
	if serviceFilter != "" {
		services = append(services, map[string]any{
			"id":          serviceFilter,
			"name":        fmt.Sprintf("projects/%s/services/%s", usedProject, serviceFilter),
			"displayName": "",
		})
	} else {
		svcIt := client.ListServices(ctx, &monitoringpb.ListServicesRequest{
			Parent: "projects/" + usedProject,
		})
		for len(services) < limit {
			svc, err := svcIt.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return errorResult(err), err
			}
			services = append(services, map[string]any{
				"name":        svc.GetName(),
				"id":          serviceIDFromName(svc.GetName()),
				"displayName": svc.GetDisplayName(),
			})
		}
	}

	totalSLOs := 0
	for _, svc := range services {
		parent := toString(svc["name"])
		objIt := client.ListServiceLevelObjectives(ctx, &monitoringpb.ListServiceLevelObjectivesRequest{
			Parent: parent,
		})
		objs := make([]map[string]any, 0)
		for {
			obj, err := objIt.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				svc["error"] = err.Error()
				break
			}
			objs = append(objs, encodeSLO(obj))
			if len(objs) >= limit {
				break
			}
		}
		svc["objectives"] = objs
		svc["objectiveCount"] = len(objs)
		totalSLOs += len(objs)
	}

	return mcp.ToolResult{Data: map[string]any{
		"project":       usedProject,
		"services":      services,
		"serviceCount":  len(services),
		"objectiveCount": totalSLOs,
	}}, nil
}

func runMQL(ctx context.Context, client *monitoring.QueryClient, project, mql string) ([]map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("monitoring query client is nil")
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
			return out, err
		}
		out = append(out, encodeTimeSeriesData(resp))
		if len(out) >= 200 {
			break
		}
	}
	return out, nil
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

func serviceIDFromName(fullName string) string {
	idx := strings.LastIndex(fullName, "/services/")
	if idx < 0 {
		return fullName
	}
	return fullName[idx+len("/services/"):]
}

func workloadCPUQuery(namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch k8s_container | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/cpu/core_usage_time' | rate(1m) | within %s",
		escape(namespace), escape(workload), durationLiteral(window),
	)
}

func workloadMemoryQuery(namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch k8s_container | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/memory/used_bytes' | within %s",
		escape(namespace), escape(workload), durationLiteral(window),
	)
}

func workloadRestartQuery(namespace, workload string, window time.Duration) string {
	return fmt.Sprintf(
		"fetch k8s_container | filter resource.namespace_name = '%s' && (resource.pod_name =~ '%s-.*') | metric 'kubernetes.io/container/restart_count' | within %s",
		escape(namespace), escape(workload), durationLiteral(window),
	)
}

func escape(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
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

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
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

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: mcp.BuildErrorEnvelope(err, nil)}
}

func schemaQuery() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"query":     map[string]any{"type": "string", "description": "Cloud Monitoring MQL query."},
		},
		"required":             []string{"query"},
		"additionalProperties": true,
	}
}

func schemaWorkload() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"namespace": map[string]any{"type": "string", "description": "Kubernetes namespace of the workload."},
			"workload":  map[string]any{"type": "string", "description": "Workload name (Deployment / StatefulSet / DaemonSet name)."},
			"duration":  map[string]any{"type": "string", "description": "Window duration (e.g. '15m', '1h'). Default 30m."},
		},
		"required":             []string{"namespace", "workload"},
		"additionalProperties": true,
	}
}

func schemaListDescriptors() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"filter":    map[string]any{"type": "string", "description": "Optional Cloud Monitoring metric descriptor filter (e.g. metric.type=starts_with(\"kubernetes.io/\"))."},
			"limit":     map[string]any{"type": "integer", "description": "Max descriptors to return (default 50, max 500)."},
		},
		"additionalProperties": true,
	}
}

func schemaSLOStatus() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "GCP project ID. Falls back to GOOGLE_CLOUD_PROJECT env or current GKE kubeconfig context."},
			"serviceId": map[string]any{"type": "string", "description": "Optional Service Monitoring service ID. When omitted, lists all services and their SLOs."},
			"limit":     map[string]any{"type": "integer", "description": "Max services and SLOs per service to enumerate (default 50, max 200)."},
		},
		"additionalProperties": true,
	}
}
