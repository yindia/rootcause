package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/audit"
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
		user, err := ctx.Policy.Authenticate(apiKey)
		if err != nil {
			logAudit(ctx, spec, "unknown", nil, nil, "error", err)
			return nil, &sdkjsonrpc.Error{Code: -32001, Message: err.Error()}
		}
		if err := ctx.Policy.AuthorizeTool(user, spec.ToolsetID, spec.Name); err != nil {
			logAudit(ctx, spec, user.ID, nil, nil, "error", err)
			return nil, &sdkjsonrpc.Error{Code: -32002, Message: err.Error()}
		}

		if ctx.Clients != nil && ctx.Config != nil {
			ttl := time.Duration(ctx.Config.Cache.DiscoveryTTLSeconds) * time.Second
			ctx.Clients.RefreshDiscovery(ttl)
		}
		execCtx, cancel := withToolTimeout(callCtx, ctx.Config, spec)
		result, toolErr := spec.Handler(execCtx, ToolRequest{Arguments: args, User: user, Context: ctx})
		cancel()
		outcome := "success"
		if toolErr != nil {
			outcome = "error"
		}
		logAudit(ctx, spec, user.ID, result.Metadata.Namespaces, result.Metadata.Resources, outcome, toolErr)

		return buildCallToolResult(result, toolErr), nil
	}
}

func buildCallToolResult(result ToolResult, toolErr error) *sdkmcp.CallToolResult {
	res := &sdkmcp.CallToolResult{}
	if len(result.Metadata.Namespaces) > 0 || len(result.Metadata.Resources) > 0 {
		res.Meta = sdkmcp.Meta{
			"namespaces": result.Metadata.Namespaces,
			"resources":  result.Metadata.Resources,
		}
	}
	if toolErr != nil {
		res.IsError = true
		res.StructuredContent = BuildErrorEnvelope(toolErr, result.Data)
		if res.Content == nil {
			res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: toolErr.Error()}}
		}
		return res
	}

	if result.Data != nil {
		res.StructuredContent = result.Data
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

func logAudit(ctx ToolContext, spec ToolSpec, userID string, namespaces, resources []string, outcome string, err error) {
	if ctx.Audit == nil {
		return
	}
	event := audit.Event{
		Timestamp:  time.Now().UTC(),
		UserID:     userID,
		Tool:       spec.Name,
		Toolset:    spec.ToolsetID,
		Namespaces: namespaces,
		Resources:  resources,
		Outcome:    outcome,
	}
	if err != nil {
		event.Error = err.Error()
	}
	ctx.Audit.Log(event)
}
