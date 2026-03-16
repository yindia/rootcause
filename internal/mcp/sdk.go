package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSDKTools(server *sdkmcp.Server, reg *ToolRegistry, ctx ToolContext) ([]string, error) {
	if server == nil || reg == nil {
		return nil, fmt.Errorf("server and registry are required")
	}
	toolNames := reg.Names()
	for _, spec := range reg.Specs() {
		schema := spec.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		tool := &sdkmcp.Tool{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: schema,
		}
		server.AddTool(tool, toolHandler(spec, ctx))
	}
	return toolNames, nil
}

func toolHandler(spec ToolSpec, ctx ToolContext) sdkmcp.ToolHandler {
	return func(callCtx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		args := map[string]any{}
		if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, &sdkjsonrpc.Error{Code: sdkjsonrpc.CodeInvalidParams, Message: fmt.Sprintf("invalid arguments: %v", err)}
			}
		}

		apiKey := apiKeyFromRequest(req)
		traceID := traceIDFromRequest(req)
		if ctx.Policy == nil {
			return nil, &sdkjsonrpc.Error{Code: -32001, Message: "policy is not configured"}
		}
		user, err := ctx.Policy.Authenticate(apiKey)
		if err != nil {
			return nil, &sdkjsonrpc.Error{Code: -32001, Message: err.Error()}
		}
		if ctx.Invoker == nil {
			return nil, &sdkjsonrpc.Error{Code: -32003, Message: "tool invoker not available"}
		}

		callCtx = withTraceID(callCtx, traceID)
		result, toolErr := ctx.Invoker.Call(callCtx, user, spec.Name, args)

		return buildCallToolResult(callCtx, result, toolErr), nil
	}
}

func buildCallToolResult(callCtx context.Context, result ToolResult, toolErr error) *sdkmcp.CallToolResult {
	res := &sdkmcp.CallToolResult{}
	meta := sdkmcp.Meta{}
	if traceID, ok := traceIDFromContext(callCtx); ok && traceID != "" {
		meta["traceId"] = traceID
	}
	if len(result.Metadata.Namespaces) > 0 {
		meta["namespaces"] = result.Metadata.Namespaces
	}
	if len(result.Metadata.Resources) > 0 {
		meta["resources"] = result.Metadata.Resources
	}
	if len(meta) > 0 {
		res.Meta = meta
	}
	if toolErr != nil {
		res.IsError = true
		if payload, ok := result.Data.(map[string]any); ok && isErrorEnvelope(payload) {
			res.StructuredContent = payload
		} else {
			res.StructuredContent = BuildErrorEnvelope(toolErr, result.Data)
		}
		if res.Content == nil {
			res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: toolErr.Error()}}
		}
		return res
	}

	if result.Data != nil {
		res.StructuredContent = result.Data
		if traceID, ok := traceIDFromContext(callCtx); ok {
			if root, ok := res.StructuredContent.(map[string]any); ok {
				if _, exists := root["traceId"]; !exists {
					root["traceId"] = traceID
				}
			}
		}
		if res.Content == nil {
			dataJSON, err := json.Marshal(result.Data)
			if err != nil {
				res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("%v", result.Data)}}
			} else {
				res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: string(dataJSON)}}
			}
		}
	} else if res.Content == nil {
		res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: "{}"}}
	}
	return res
}

func apiKeyFromRequest(req *sdkmcp.CallToolRequest) string {
	if req == nil {
		return ""
	}
	if req.Params != nil {
		if value := apiKeyFromMeta(req.Params.Meta); value != "" {
			return value
		}
	}
	if req.Extra != nil && req.Extra.Header != nil {
		if value := strings.TrimSpace(req.Extra.Header.Get("X-Api-Key")); value != "" {
			return value
		}
		authHeader := strings.TrimSpace(req.Extra.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			return strings.TrimSpace(authHeader[len("bearer "):])
		}
	}
	return ""
}

func apiKeyFromMeta(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta["apiKey"].(string); ok {
		return value
	}
	if auth, ok := meta["auth"].(map[string]any); ok {
		if value, ok := auth["apiKey"].(string); ok {
			return value
		}
	}
	return ""
}

func traceIDFromRequest(req *sdkmcp.CallToolRequest) string {
	if req == nil {
		return ""
	}
	if req.Params != nil {
		if traceID := traceIDFromMeta(req.Params.Meta); traceID != "" {
			return traceID
		}
	}
	if req.Extra != nil && req.Extra.Header != nil {
		if traceID := strings.TrimSpace(req.Extra.Header.Get("X-Trace-Id")); traceID != "" {
			return traceID
		}
	}
	return ""
}

func traceIDFromMeta(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if value, ok := meta["traceId"].(string); ok {
		return strings.TrimSpace(value)
	}
	if value, ok := meta["traceID"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
