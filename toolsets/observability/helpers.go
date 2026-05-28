package observability

import (
	"strings"
	"time"

	"rootcause/internal/mcp"
)

func argString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func argInt(m map[string]any, key string, fallback int) int {
	if m == nil {
		return fallback
	}
	switch t := m[key].(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	}
	return fallback
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

func errorResult(err error) mcp.ToolResult {
	return mcp.ToolResult{Data: mcp.BuildErrorEnvelope(err, nil)}
}

// resourceTypeOrDefault returns the user-provided resource type when set,
// else "k8s_container" — the default echoed back in tool responses so
// operators can see exactly which monitored resource was queried.
func resourceTypeOrDefault(rt string) string {
	if s := strings.TrimSpace(rt); s != "" {
		return s
	}
	return "k8s_container"
}
