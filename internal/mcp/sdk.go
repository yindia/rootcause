package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/xeipuuv/gojsonschema"
)

func RegisterSDKTools(server *sdkmcp.Server, inv *ToolInvoker) ([]string, error) {
	if server == nil || inv == nil {
		return nil, fmt.Errorf("server and invoker are required")
	}
	reg, _, ok := inv.Runtime()
	if !ok || reg == nil {
		return nil, fmt.Errorf("invoker runtime not initialized")
	}
	for _, spec := range reg.Specs() {
		AddSDKTool(server, spec, inv)
	}
	return reg.Names(), nil
}

func AddSDKTool(server *sdkmcp.Server, spec ToolSpec, inv *ToolInvoker) {
	server.AddTool(&sdkmcp.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: spec.AugmentedSchema(),
	}, toolHandler(spec, inv))
}

func schemaWithGlobalSkillTags(schema map[string]any) map[string]any {
	out := map[string]any{}
	maps.Copy(out, schema)
	props := map[string]any{}
	if existing, ok := schema["properties"].(map[string]any); ok {
		maps.Copy(props, existing)
	}
	props["skillTags"] = map[string]any{
		"description": "Optional custom skill tags to include for this call.",
		"oneOf": []map[string]any{
			{"type": "string"},
			{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	props["customSkillTags"] = map[string]any{
		"description": "Alias for skillTags; matches configured custom skills by tag.",
		"oneOf": []map[string]any{
			{"type": "string"},
			{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}
	out["properties"] = props
	return out
}

func toolHandler(spec ToolSpec, inv *ToolInvoker) sdkmcp.ToolHandler {
	return func(callCtx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		args := map[string]any{}
		if req != nil && req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, &sdkjsonrpc.Error{Code: sdkjsonrpc.CodeInvalidParams, Message: fmt.Sprintf("invalid arguments: %v", err)}
			}
		}

		_, tctx, ok := inv.Runtime()
		if !ok {
			return nil, &sdkjsonrpc.Error{Code: -32003, Message: "tool invoker not initialized"}
		}
		apiKey := apiKeyFromRequest(req)
		traceID := traceIDFromRequest(req)
		if tctx.Policy == nil {
			return nil, &sdkjsonrpc.Error{Code: -32001, Message: "policy is not configured"}
		}
		user, err := tctx.Policy.Authenticate(apiKey)
		if err != nil {
			return nil, &sdkjsonrpc.Error{Code: -32001, Message: err.Error()}
		}

		callCtx = withTraceID(callCtx, traceID)

		if tctx.Config != nil && tctx.Config.Limits.StrictSchema {
			if schema, schemaErr := spec.CompileSchema(); schemaErr == nil && schema != nil {
				validation, vErr := schema.Validate(gojsonschema.NewGoLoader(args))
				if vErr != nil || (validation != nil && !validation.Valid()) {
					detail := schemaValidationErrors(validation, vErr)
					envelope := BuildErrorEnvelope(fmt.Errorf("invalid arguments: %s", strings.Join(detail, "; ")), map[string]any{"tool": spec.Name, "violations": detail})
					result := ToolResult{Data: envelope}
					return buildCallToolResult(callCtx, result, fmt.Errorf("invalid arguments"), tctx.Config.Limits.MaxResultBytes), nil
				}
			}
		}

		result, toolErr := inv.Call(callCtx, user, spec.Name, args)

		maxBytes := 0
		if tctx.Config != nil {
			maxBytes = tctx.Config.Limits.MaxResultBytes
		}
		return buildCallToolResult(callCtx, result, toolErr, maxBytes), nil
	}
}

func schemaValidationErrors(result *gojsonschema.Result, err error) []string {
	if err != nil {
		return []string{err.Error()}
	}
	if result == nil {
		return nil
	}
	out := make([]string, 0, len(result.Errors()))
	for _, e := range result.Errors() {
		out = append(out, e.String())
	}
	return out
}

const truncationNotice = "\n... [truncated: result exceeds max_result_bytes; full payload available in StructuredContent]"

func buildCallToolResult(callCtx context.Context, result ToolResult, toolErr error, maxBytes int) *sdkmcp.CallToolResult {
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
	if len(result.Metadata.CustomSkills) > 0 {
		meta["customSkillGuidance"] = result.Metadata.CustomSkills
	}
	if result.Metadata.CustomSkillError != "" {
		meta["customSkillError"] = result.Metadata.CustomSkillError
	}
	if len(meta) > 0 {
		res.Meta = meta
	}
	if toolErr != nil {
		res.IsError = true
		if IsErrorEnvelope(result.Data) {
			res.StructuredContent = result.Data
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
			switch {
			case err != nil:
				res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: fmt.Sprintf("%v", result.Data)}}
			case maxBytes > 0 && len(dataJSON) > maxBytes:
				if res.Meta == nil {
					res.Meta = sdkmcp.Meta{}
				}
				res.Meta["truncated"] = true
				res.Meta["originalBytes"] = len(dataJSON)
				res.Content = []sdkmcp.Content{&sdkmcp.TextContent{Text: string(dataJSON[:maxBytes]) + truncationNotice}}
			default:
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
