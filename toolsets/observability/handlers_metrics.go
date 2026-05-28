package observability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rootcause/internal/mcp"
)

func (t *Toolset) handleMetricsQuery(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	query := strings.TrimSpace(argString(req.Arguments, "query"))
	if query == "" {
		err := fmt.Errorf("query is required")
		return errorResult(err), err
	}
	series, usedProject, err := backend.Metrics().RawQuery(ctx, project, query)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"backend":    backend.Name(),
		"project":    usedProject,
		"query":      query,
		"timeSeries": series,
		"count":      len(series),
	}}, nil
}

func (t *Toolset) handleMetricsWorkload(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
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
	resourceType := strings.TrimSpace(argString(req.Arguments, "resourceType"))
	window := parseDuration(argString(req.Arguments, "duration"), 30*time.Minute)
	res, err := backend.Metrics().WorkloadMetrics(ctx, project, resourceType, namespace, workload, window)
	if err != nil {
		return errorResult(err), err
	}
	out := map[string]any{
		"backend":      backend.Name(),
		"project":      res.Project,
		"namespace":    namespace,
		"workload":     workload,
		"resourceType": resourceTypeOrDefault(resourceType),
		"window":       window.String(),
		"metrics": map[string]any{
			"cpu":          res.CPU,
			"memory":       res.Memory,
			"restartCount": res.RestartCount,
		},
	}
	if len(res.Errors) > 0 {
		out["errors"] = res.Errors
	}
	resources := []string{fmt.Sprintf("%s/%s", namespace, workload)}
	return mcp.ToolResult{
		Data:     out,
		Metadata: mcp.ToolMetadata{Namespaces: []string{namespace}, Resources: resources},
	}, nil
}

func (t *Toolset) handleListDescriptors(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	filter := strings.TrimSpace(argString(req.Arguments, "filter"))
	limit := argInt(req.Arguments, "limit", 50)
	descriptors, usedProject, err := backend.Metrics().ListDescriptors(ctx, project, filter, limit)
	if err != nil {
		return errorResult(err), err
	}
	return mcp.ToolResult{Data: map[string]any{
		"backend":     backend.Name(),
		"project":     usedProject,
		"filter":      filter,
		"descriptors": descriptors,
		"count":       len(descriptors),
	}}, nil
}

func (t *Toolset) handleSLOList(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	backend, err := t.requireBackend()
	if err != nil {
		return errorResult(err), err
	}
	project := argString(req.Arguments, "projectId")
	serviceFilter := strings.TrimSpace(argString(req.Arguments, "serviceId"))
	limit := argInt(req.Arguments, "limit", 50)
	metrics := backend.Metrics()

	services := make([]map[string]any, 0)
	var usedProject string
	if serviceFilter != "" {
		// Resolve project (via ListServices with a tiny page) then build the
		// canonical service name. Keeps a single resolution code path.
		_, resolved, lookupErr := metrics.ListServices(ctx, project, 1)
		if lookupErr != nil {
			return errorResult(lookupErr), lookupErr
		}
		usedProject = resolved
		services = []map[string]any{{
			"id":          serviceFilter,
			"name":        fmt.Sprintf("projects/%s/services/%s", usedProject, serviceFilter),
			"displayName": "",
		}}
	} else {
		var listErr error
		services, usedProject, listErr = metrics.ListServices(ctx, project, limit)
		if listErr != nil {
			return errorResult(listErr), listErr
		}
	}

	totalSLOs := 0
	for _, svc := range services {
		parent := argString(svc, "name")
		objs, err := metrics.ListSLOs(ctx, parent, limit)
		if err != nil {
			svc["error"] = err.Error()
			continue
		}
		svc["objectives"] = objs
		svc["objectiveCount"] = len(objs)
		totalSLOs += len(objs)
	}

	return mcp.ToolResult{Data: map[string]any{
		"backend":        backend.Name(),
		"project":        usedProject,
		"services":       services,
		"serviceCount":   len(services),
		"objectiveCount": totalSLOs,
	}}, nil
}

func schemaMetricsQuery() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string", "description": "Optional project ID (interpretation depends on backend; falls back to GOOGLE_CLOUD_PROJECT / observability.gcp.project)."},
			"query":     map[string]any{"type": "string", "description": "Backend-native query (MQL for GCP)."},
		},
		"required":             []string{"query"},
		"additionalProperties": true,
	}
}

func schemaMetricsWorkload() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId":    map[string]any{"type": "string", "description": "Optional project ID."},
			"namespace":    map[string]any{"type": "string", "description": "Kubernetes namespace."},
			"workload":     map[string]any{"type": "string", "description": "Workload name."},
			"resourceType": map[string]any{"type": "string", "description": "Monitored resource type (default k8s_container). Use e.g. generic_node for self-managed clusters routing telemetry via the Ops Agent."},
			"duration":     map[string]any{"type": "string", "description": "Window duration. Default 30m."},
		},
		"required":             []string{"namespace", "workload"},
		"additionalProperties": true,
	}
}

func schemaListDescriptors() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string"},
			"filter":    map[string]any{"type": "string", "description": "Backend-native metric descriptor filter."},
			"limit":     map[string]any{"type": "integer", "description": "Max descriptors (default 50, max 500)."},
		},
		"additionalProperties": true,
	}
}

func schemaSLOList() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"projectId": map[string]any{"type": "string"},
			"serviceId": map[string]any{"type": "string", "description": "Optional service ID to scope SLO enumeration."},
			"limit":     map[string]any{"type": "integer", "description": "Max services / SLOs per service (default 50, max 200)."},
		},
		"additionalProperties": true,
	}
}
