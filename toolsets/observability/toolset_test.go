package observability

import (
	"context"
	"strings"
	"testing"
	"time"

	"rootcause/internal/config"
	"rootcause/internal/mcp"
)

func TestNewToolsetIdentity(t *testing.T) {
	ts := New()
	if ts.ID() != "observability" {
		t.Fatalf("expected ID 'observability', got %q", ts.ID())
	}
	if ts.Version() == "" {
		t.Fatalf("expected non-empty version")
	}
}

func TestRegisterExposesAllTools(t *testing.T) {
	ts := New()
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	if err := ts.Init(mcp.ToolContext{Config: &cfg}); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := ts.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	for _, want := range []string{
		"observability.metrics.query",
		"observability.metrics.workload",
		"observability.metrics.list_descriptors",
		"observability.metrics.slo_list",
		"observability.logs.query",
		"observability.logs.workload",
		"observability.logs.error_timeline",
		"observability.logs.correlated_with_bundle",
	} {
		if _, ok := reg.Get(want); !ok {
			t.Errorf("expected tool %q to be registered", want)
		}
	}
}

func TestSelectBackendReturnsNilWithoutConfig(t *testing.T) {
	// Empty config = no backend. Tools should still register (so capabilities
	// list them) but handlers surface a clean "no backend configured" error.
	if got := selectBackend(mcp.ToolContext{}); got != nil {
		t.Fatalf("expected nil backend with empty config, got %T", got)
	}
}

func TestSelectBackendPicksGCPWhenConfigured(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Observability.GCP.Project = "my-ops-proj"
	if got := selectBackend(mcp.ToolContext{Config: &cfg}); got == nil || got.Name() != "gcp" {
		t.Fatalf("expected gcp backend, got %#v", got)
	}
}

func TestSelectBackendPicksGCPFromCredentialsAlone(t *testing.T) {
	// A credentials file without an explicit project is still enough to
	// attempt the backend: explicit projectId args on tool calls can supply
	// the project. Be permissive at registration, strict at call time.
	cfg := config.DefaultConfig()
	cfg.GCP.CredentialsFile = "/keys/sa.json"
	if got := selectBackend(mcp.ToolContext{Config: &cfg}); got == nil {
		t.Fatalf("expected gcp backend when credentials_file is set")
	}
}

func TestHandlerReturnsCleanErrorWithoutBackend(t *testing.T) {
	ts := New()
	// Init with empty config so backend stays nil.
	if err := ts.Init(mcp.ToolContext{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	_, err := ts.handleMetricsWorkload(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"namespace": "payments",
		"workload":  "api",
	}})
	if err == nil || !strings.Contains(err.Error(), "no observability backend") {
		t.Fatalf("expected 'no observability backend' error, got %v", err)
	}
}

func TestWorkloadQueriesHonorResourceType(t *testing.T) {
	// Sanity-check the GCP query builders accept a configurable resource type
	// AND fall back to k8s_container when an unsafe value is passed. This is
	// the parameter rollout that lets self-managed clusters using generic_node
	// fetch workload metrics + logs via the observability tools.
	for _, in := range []string{"generic_node", "k8s_container"} {
		mql := workloadCPUQuery(mqlResourceLiteral(in), "payments", "api", 30*time.Minute)
		want := "fetch " + in + " | filter resource.namespace_name"
		if !strings.Contains(mql, want) {
			t.Errorf("workloadCPUQuery missing %q in %s", want, mql)
		}
		filter := workloadFilter(filterResourceLiteral(in), "payments", "api", "ERROR", 30*time.Minute)
		want = `resource.type="` + in + `"`
		if !strings.Contains(filter, want) {
			t.Errorf("workloadFilter missing %q in %s", want, filter)
		}
	}
	// Injection attempt → falls back to k8s_container.
	mql := workloadCPUQuery(mqlResourceLiteral("k8s_container | delete"), "p", "a", time.Minute)
	if !strings.Contains(mql, "fetch k8s_container ") {
		t.Errorf("expected injection to fall back to k8s_container, got %s", mql)
	}
}
