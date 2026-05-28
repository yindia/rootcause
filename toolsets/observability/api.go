// Package observability is the vendor-neutral observability toolset. Tools
// registered here delegate to a configured Backend (GCP Stackdriver today;
// Prometheus and others planned). Other toolsets (e.g. rootcause.incident_bundle)
// call observability.* tools rather than vendor-specific ones, so swapping
// backends does not ripple through callers.
package observability

import (
	"context"
	"time"
)

// Backend is the integration point for a concrete observability provider.
// Implementations live in this package (backend_gcp.go) but conceptually any
// vendor with metrics + logs APIs can be plugged in.
type Backend interface {
	// Name returns a stable identifier for the backend ("gcp", "prometheus", …).
	// Surfaced in tool responses so operators can verify which backend served
	// the data.
	Name() string

	// Metrics returns the metrics API. Implementations MAY return nil if the
	// backend has no metrics support; handlers translate that into a clear
	// "metrics not supported by backend" tool error.
	Metrics() MetricsAPI

	// Logs returns the logs API. May return nil similar to Metrics.
	Logs() LogsAPI
}

// MetricsAPI is the domain-shaped (not query-language-shaped) metrics surface.
// Backend implementations construct the right query for their vendor.
type MetricsAPI interface {
	// WorkloadMetrics fetches CPU / memory / restart counts for a Kubernetes
	// workload across the window. resourceType identifies the monitored
	// resource (k8s_container for GKE, generic_node for Ops Agent setups,
	// etc.); pass "" to use the backend's default. Backends that don't model
	// workloads natively (Prometheus) can rely on standard recording rules.
	WorkloadMetrics(ctx context.Context, project, resourceType, namespace, workload string, window time.Duration) (out WorkloadMetricsResult, err error)

	// ListDescriptors returns metric descriptors matching the backend-native
	// filter (e.g. Cloud Monitoring filter, Prometheus label matcher).
	ListDescriptors(ctx context.Context, project, filter string, limit int) (descriptors []map[string]any, usedProject string, err error)

	// ListServices enumerates configured services (used by SLO tooling). May
	// return an empty slice if the backend has no equivalent.
	ListServices(ctx context.Context, project string, limit int) (services []map[string]any, usedProject string, err error)

	// ListSLOs returns Service Level Objectives for a fully-qualified service
	// name (semantics defined by the backend).
	ListSLOs(ctx context.Context, serviceName string, limit int) ([]map[string]any, error)

	// RawQuery runs a backend-native query (MQL for GCP, PromQL for
	// Prometheus). Used by the observability.metrics.query escape hatch.
	RawQuery(ctx context.Context, project, query string) (series []map[string]any, usedProject string, err error)
}

// WorkloadMetricsResult is the normalized return shape for WorkloadMetrics.
// Each field is a slice of time-series points in backend-encoded form (the
// handler returns them verbatim under the tool result's "metrics" key).
type WorkloadMetricsResult struct {
	Project      string
	CPU          []map[string]any
	Memory       []map[string]any
	RestartCount []map[string]any
	Errors       map[string]string // per-series error message; non-nil for partials
}

// LogsAPI is the domain-shaped logs surface.
type LogsAPI interface {
	// WorkloadEntries fetches recent log entries for a workload at or above the
	// given severity. resourceType selects the monitored resource (pass ""
	// for the backend default). Returns the encoded entries plus the final
	// backend-native filter used (for transparency).
	WorkloadEntries(ctx context.Context, project, resourceType, namespace, workload, severity string, window time.Duration, limit int) (out WorkloadEntriesResult, err error)

	// EntriesInWindow pulls log entries inside an explicit time window —
	// used by the bundle-correlation tool. resourceType is plumbed for the
	// same reason as WorkloadEntries.
	EntriesInWindow(ctx context.Context, project, resourceType, namespace, workload, severity string, start, end time.Time, limit int) (out WorkloadEntriesResult, err error)

	// BucketedErrorCounts returns per-bucket / per-severity counts using the
	// backend's native counting primitive (log_entry_count metric on GCP,
	// rate() on Prometheus). resourceType identifies the monitored resource
	// (k8s_container, generic_node, …).
	BucketedErrorCounts(ctx context.Context, project, resourceType, namespace, workload string, severities []string, window, bucket time.Duration) (out BucketedCountsResult, err error)

	// RawQuery runs a backend-native log query (Cloud Logging filter for GCP,
	// LogQL for Loki). Powers observability.logs.query.
	RawQuery(ctx context.Context, project, filter string, window time.Duration, limit int) (out WorkloadEntriesResult, err error)
}

// WorkloadEntriesResult is the normalized return shape for log queries.
type WorkloadEntriesResult struct {
	Project string
	Filter  string
	Entries []map[string]any
}

// BucketedCountsResult is the normalized return shape for error_timeline.
type BucketedCountsResult struct {
	Project        string
	Query          string         // backend-native query (e.g. MQL) for transparency
	Buckets        []map[string]any
	TotalCount     int
	SeverityCounts map[string]int
}
