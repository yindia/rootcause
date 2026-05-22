package mcp

import (
	"context"
	"testing"
	"time"

	"rootcause/internal/config"
)

func TestToolTimeoutDefaults(t *testing.T) {
	cfg := config.DefaultConfig()
	timeout := toolTimeout(&cfg, "k8s.get")
	if timeout <= 0 {
		t.Fatalf("expected default timeout to be set")
	}
}

func TestToolTimeoutPerTool(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Timeouts.PerTool = map[string]int{"k8s.get": 12}
	timeout := toolTimeout(&cfg, "k8s.get")
	if timeout != 12*time.Second {
		t.Fatalf("expected per-tool timeout, got %s", timeout)
	}
}

func TestToolTimeoutMaxCap(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Timeouts.DefaultSeconds = 120
	cfg.Timeouts.MaxSeconds = 30
	timeout := toolTimeout(&cfg, "k8s.get")
	if timeout != 30*time.Second {
		t.Fatalf("expected max-capped timeout, got %s", timeout)
	}
}

func TestToolTimeoutNilAndNegative(t *testing.T) {
	if toolTimeout(nil, "k8s.get") != 0 {
		t.Fatalf("expected zero timeout for nil config")
	}
	cfg := config.DefaultConfig()
	cfg.Timeouts.DefaultSeconds = -1
	if toolTimeout(&cfg, "k8s.get") != 0 {
		t.Fatalf("expected zero timeout for negative default")
	}
	cfg.Timeouts.DefaultSeconds = 0
	cfg.Timeouts.MaxSeconds = 15
	if toolTimeout(&cfg, "k8s.get") != 15*time.Second {
		t.Fatalf("expected max timeout when default zero, got %s", toolTimeout(&cfg, "k8s.get"))
	}
}

func TestWithToolTimeoutNoop(t *testing.T) {
	ctx, cancel := withToolTimeout(context.Background(), nil, ToolSpec{Name: "k8s.get"})
	cancel()
	if ctx == nil {
		t.Fatalf("expected context")
	}
}

func TestWithToolTimeoutCappedByParent(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Timeouts.PerTool = map[string]int{"child.tool": 60}
	parent, cancelParent := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelParent()
	child, cancelChild := withToolTimeout(parent, &cfg, ToolSpec{Name: "child.tool"})
	defer cancelChild()
	deadline, ok := child.Deadline()
	if !ok {
		t.Fatalf("expected child deadline")
	}
	remaining := time.Until(deadline)
	if remaining > 250*time.Millisecond {
		t.Fatalf("child deadline %s exceeds parent budget", remaining)
	}
}
