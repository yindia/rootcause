package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"rootcause/internal/mcp"
)

func (t *Toolset) wrapListCache(spec mcp.ToolSpec) mcp.ToolSpec {
	if t.ctx.Cache == nil || t.ctx.Config == nil {
		return spec
	}
	if !strings.Contains(spec.Name, ".list_") {
		return spec
	}
	ttlSeconds := t.ctx.Config.Cache.AWSListTTLSeconds
	if ttlSeconds <= 0 {
		return spec
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	handler := spec.Handler
	spec.Handler = func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
		key := awsListCacheKey(spec.Name, req.Arguments)
		if cached, ok := t.ctx.Cache.Get(key); ok {
			return mcp.ToolResult{Data: cached}, nil
		}
		result, err := handler(ctx, req)
		if err == nil && result.Data != nil {
			t.ctx.Cache.Set(key, result.Data, ttl)
		}
		return result, err
	}
	return spec
}

func awsListCacheKey(toolName string, args map[string]any) string {
	return fmt.Sprintf("awslist:%s:%s", toolName, stableValue(args))
}

func stableValue(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, stableValue(typed[key])))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case map[string]string:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, typed[key]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, stableValue(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case []string:
		return "[" + strings.Join(typed, ",") + "]"
	case string:
		return strings.TrimSpace(typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
