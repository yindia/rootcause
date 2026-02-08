package mcp

import (
	"context"
	"time"

	"rootcause/internal/config"
)

func withToolTimeout(ctx context.Context, cfg *config.Config, spec ToolSpec) (context.Context, context.CancelFunc) {
	if cfg == nil {
		return ctx, func() {}
	}
	timeout := toolTimeout(cfg, spec.Name)
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func toolTimeout(cfg *config.Config, toolName string) time.Duration {
	if cfg == nil {
		return 0
	}
	timeout := time.Duration(cfg.Timeouts.DefaultSeconds) * time.Second
	if cfg.Timeouts.PerTool != nil {
		if override, ok := cfg.Timeouts.PerTool[toolName]; ok && override > 0 {
			timeout = time.Duration(override) * time.Second
		}
	}
	max := time.Duration(cfg.Timeouts.MaxSeconds) * time.Second
	if max > 0 && timeout > max {
		timeout = max
	}
	if timeout < 0 {
		return 0
	}
	if timeout == 0 && max > 0 {
		return max
	}
	return timeout
}
