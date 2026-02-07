package mcp

import (
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
