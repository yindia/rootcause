package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	sdkjsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/audit"
	"rootcause/internal/config"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestAPIKeyFromMeta(t *testing.T) {
	meta := map[string]any{"apiKey": "abc"}
	if apiKeyFromMeta(meta) != "abc" {
		t.Fatalf("expected api key from meta")
	}
	meta = map[string]any{"auth": map[string]any{"apiKey": "def"}}
	if apiKeyFromMeta(meta) != "def" {
		t.Fatalf("expected api key from auth")
	}
}

func TestAPIKeyFromRequest(t *testing.T) {
	req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Meta: map[string]any{"apiKey": "xyz"}}}
	if apiKeyFromRequest(req) != "xyz" {
		t.Fatalf("expected api key from request meta")
	}

	req.Extra = &sdkmcp.RequestExtra{Header: http.Header{"X-Api-Key": []string{"header-key"}}}
	if apiKeyFromRequest(req) != "xyz" {
		t.Fatalf("expected meta to win over header")
	}

	req = &sdkmcp.CallToolRequest{Extra: &sdkmcp.RequestExtra{Header: http.Header{"Authorization": []string{"Bearer token"}}}}
	if apiKeyFromRequest(req) != "token" {
		t.Fatalf("expected bearer token from header")
	}

	if apiKeyFromRequest(nil) != "" {
		t.Fatalf("expected empty api key for nil request")
	}
}

func TestRegisterSDKToolsAndToolHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := NewRegistry(&cfg)
	called := false
	spec := ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		InputSchema: map[string]any{
			"type": "object",
		},
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			called = true
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	}
	_ = reg.Add(spec)
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rootcause", Version: "test"}, nil)
	toolCtx := ToolContext{
		Config:   &cfg,
		Policy:   policy.NewAuthorizer(),
		Renderer: render.NewRenderer(),
		Redactor: redact.New(),
		Audit:    audit.NewLogger(io.Discard),
	}
	tools, err := RegisterSDKTools(server, reg, toolCtx)
	if err != nil {
		t.Fatalf("register tools: %v", err)
	}
	if len(tools) != 1 || tools[0] != "demo" {
		t.Fatalf("unexpected tools list: %#v", tools)
	}

	handler := toolHandler(spec, toolCtx)
	args, _ := json.Marshal(map[string]any{"name": "ok"})
	req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "demo", Arguments: args}}
	_, err = handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !called {
		t.Fatalf("expected handler to be called")
	}
}

func TestRegisterSDKToolsNilArgs(t *testing.T) {
	if _, err := RegisterSDKTools(nil, nil, ToolContext{}); err == nil {
		t.Fatalf("expected error for nil server/registry")
	}
}

func TestBuildCallToolResultSuccess(t *testing.T) {
	result := ToolResult{
		Data: map[string]any{"ok": true},
		Metadata: ToolMetadata{
			Namespaces: []string{"default"},
		},
	}
	out := buildCallToolResult(result, nil)
	if out.StructuredContent == nil {
		t.Fatalf("expected structured content")
	}
	if out.Meta["namespaces"] == nil {
		t.Fatalf("expected namespaces meta")
	}
}

func TestBuildCallToolResultError(t *testing.T) {
	err := errors.New("boom")
	result := ToolResult{Data: map[string]any{"hint": "test"}}
	out := buildCallToolResult(result, err)
	if !out.IsError {
		t.Fatalf("expected error result")
	}
	payload, ok := out.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected map content")
	}
	if _, ok := payload["error"]; !ok {
		t.Fatalf("expected error envelope")
	}
}

func TestBuildCallToolResultFallbacks(t *testing.T) {
	out := buildCallToolResult(ToolResult{}, nil)
	if out.Content == nil || len(out.Content) == 0 {
		t.Fatalf("expected content for empty result")
	}
	result := ToolResult{Data: map[string]any{"bad": func() {}}}
	out = buildCallToolResult(result, nil)
	if out.Content == nil || len(out.Content) == 0 {
		t.Fatalf("expected content fallback for marshal error")
	}
}

func TestToolHandlerInvalidArgs(t *testing.T) {
	cfg := config.DefaultConfig()
	spec := ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{}, nil
		},
	}
	toolCtx := ToolContext{
		Config: &cfg,
		Policy: policy.NewAuthorizer(),
	}
	handler := toolHandler(spec, toolCtx)
	req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "demo", Arguments: []byte("{")}}
	_, err := handler(context.Background(), req)
	if err == nil {
		t.Fatalf("expected error for invalid args")
	}
	if _, ok := err.(*sdkjsonrpc.Error); !ok {
		t.Fatalf("expected jsonrpc error, got %T", err)
	}
}

func TestToolHandlerErrorResult(t *testing.T) {
	cfg := config.DefaultConfig()
	spec := ToolSpec{
		Name:      "demo",
		ToolsetID: "core",
		Handler: func(ctx context.Context, req ToolRequest) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"hint": "fail"}}, errors.New("fail")
		},
	}
	toolCtx := ToolContext{
		Config: &cfg,
		Policy: policy.NewAuthorizer(),
		Audit:  audit.NewLogger(io.Discard),
	}
	handler := toolHandler(spec, toolCtx)
	req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "demo"}}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected error result")
	}
}

func TestLogAuditWritesEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := audit.NewLogger(&buf)
	spec := ToolSpec{Name: "k8s.get", ToolsetID: "k8s"}
	logAudit(ToolContext{Audit: logger}, spec, "user", []string{"default"}, []string{"pods/default/p1"}, "success", nil)
	if !strings.Contains(buf.String(), `"tool":"k8s.get"`) {
		t.Fatalf("expected audit output, got %s", buf.String())
	}
}
