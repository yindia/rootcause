package gcp

import (
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestNewToolset(t *testing.T) {
	ts := New()
	if ts.ID() != "gcp" {
		t.Fatalf("expected ID gcp, got %q", ts.ID())
	}
	if ts.Version() == "" {
		t.Fatalf("expected non-empty version")
	}
}

func TestToolsetInitWithoutClients(t *testing.T) {
	// The GCP toolset uses only GCP APIs, so it must initialize without kube
	// clients (e.g. an EKS/AKS cluster shipping telemetry to GCP, or a
	// workstation with no kubeconfig).
	ts := New()
	if err := ts.Init(mcp.ToolContext{}); err != nil {
		t.Fatalf("expected GCP toolset to init without kube clients, got: %v", err)
	}
}

func TestToolsetRegistersExpectedTools(t *testing.T) {
	ts := New()
	if err := ts.Init(mcp.ToolContext{Clients: &kube.Clients{}}); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	if err := ts.Register(reg); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	expected := []string{
		"gcp.metrics.query",
		"gcp.metrics.workload",
		"gcp.metrics.list_descriptors",
		"gcp.metrics.slo_list",
		"gcp.logs.query",
		"gcp.logs.workload",
		"gcp.logs.error_timeline",
		"gcp.logs.correlated_with_bundle",
	}
	for _, name := range expected {
		if _, ok := reg.Get(name); !ok {
			t.Fatalf("expected tool %s to be registered", name)
		}
	}
}

func TestProjectResolutionRequired(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("GCP_PROJECT", "")
	ts := New()
	if err := ts.Init(mcp.ToolContext{Clients: &kube.Clients{}}); err != nil {
		t.Fatalf("init: %v", err)
	}
	_, _, err := ts.queryClient(t.Context(), "")
	if err == nil {
		t.Fatalf("expected error when project id is missing")
	}
}
