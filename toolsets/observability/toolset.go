package observability

import (
	"errors"
	"fmt"
	"strings"

	"rootcause/internal/mcp"
)

// Toolset registers vendor-neutral observability tools and dispatches them
// through a configured Backend. Today the only backend is GCP Stackdriver;
// future backends (Prometheus + Loki, CloudWatch, Datadog) implement the same
// Backend interface and plug in via selectBackend.
type Toolset struct {
	ctx     mcp.ToolContext
	backend Backend
}

func New() *Toolset { return &Toolset{} }

func init() {
	mcp.MustRegisterToolset("observability", func() mcp.Toolset { return New() })
}

func (t *Toolset) ID() string      { return "observability" }
func (t *Toolset) Version() string { return "0.1.0" }

func (t *Toolset) Init(ctx mcp.ToolContext) error {
	t.ctx = ctx
	t.backend = selectBackend(ctx)
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	// Register all observability.* tools unconditionally so they appear in
	// capabilities even before a backend is configured. Handlers return a
	// clear "no backend configured" error when t.backend is nil — easier to
	// debug than a missing tool.
	tools := []mcp.ToolSpec{
		{
			Name:        "observability.metrics.query",
			Description: "Run a raw query against the active observability backend (MQL for GCP).",
			ToolsetID:   t.ID(),
			InputSchema: schemaMetricsQuery(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleMetricsQuery,
		},
		{
			Name:        "observability.metrics.workload",
			Description: "Fetch CPU, memory, and restart count metrics for a Kubernetes workload over a time window.",
			ToolsetID:   t.ID(),
			InputSchema: schemaMetricsWorkload(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleMetricsWorkload,
		},
		{
			Name:        "observability.metrics.list_descriptors",
			Description: "List metric descriptors for discoverability (backend-native filter).",
			ToolsetID:   t.ID(),
			InputSchema: schemaListDescriptors(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleListDescriptors,
		},
		{
			Name:        "observability.metrics.slo_list",
			Description: "Enumerate services and their Service Level Objectives.",
			ToolsetID:   t.ID(),
			InputSchema: schemaSLOList(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleSLOList,
		},
		{
			Name:        "observability.logs.query",
			Description: "Run a raw log filter against the active observability backend.",
			ToolsetID:   t.ID(),
			InputSchema: schemaLogsQuery(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleLogsQuery,
		},
		{
			Name:        "observability.logs.workload",
			Description: "Fetch recent errors/warnings for a Kubernetes workload over a time window.",
			ToolsetID:   t.ID(),
			InputSchema: schemaLogsWorkload(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleLogsWorkload,
		},
		{
			Name:        "observability.logs.error_timeline",
			Description: "Bucketed error counts over a window for a workload (or filter).",
			ToolsetID:   t.ID(),
			InputSchema: schemaErrorTimeline(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleErrorTimeline,
		},
		{
			Name:        "observability.logs.correlated_with_bundle",
			Description: "Pull log entries matching a rootcause incident bundle's event window.",
			ToolsetID:   t.ID(),
			InputSchema: schemaCorrelatedWithBundle(),
			Safety:      mcp.SafetyReadOnly,
			Handler:     t.handleCorrelatedWithBundle,
		},
	}
	for _, spec := range tools {
		if err := reg.Add(spec); err != nil {
			return fmt.Errorf("register %s: %w", spec.Name, err)
		}
	}
	return nil
}

// selectBackend reads config and constructs the configured backend. Returns
// nil when no backend section is set; handlers then surface a clear error per
// call so users know they need to configure one.
func selectBackend(ctx mcp.ToolContext) Backend {
	if ctx.Config == nil {
		return nil
	}
	obs := ctx.Config.Observability.GCP
	hasProject := strings.TrimSpace(obs.Project) != ""
	// Even without a project field, a credentials_file alone is enough to
	// attempt the backend — explicit projectId args on tool calls can supply
	// the rest. Be permissive at registration; strict at call time.
	hasCreds := strings.TrimSpace(obs.CredentialsFile) != "" || strings.TrimSpace(ctx.Config.GCP.CredentialsFile) != ""
	if !hasProject && !hasCreds {
		return nil
	}
	credsFile := obs.CredentialsFile
	if credsFile == "" {
		credsFile = ctx.Config.GCP.CredentialsFile
	}
	return newGCPBackend(obs.Project, credsFile)
}

// requireBackend is a small helper used by every handler to fail cleanly when
// no backend was configured. Returning a tool-call error here is much friendlier
// than a panic / nil-pointer further down.
func (t *Toolset) requireBackend() (Backend, error) {
	if t.backend == nil {
		return nil, errors.New("no observability backend configured: set observability.gcp.project in config.yaml")
	}
	return t.backend, nil
}
