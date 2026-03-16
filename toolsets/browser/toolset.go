package browser

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"rootcause/internal/mcp"
)

type Toolset struct {
	ctx     mcp.ToolsetContext
	enabled bool
}

type toolDef struct {
	name        string
	description string
	safety      mcp.ToolSafety
	schema      map[string]any
	handler     mcp.ToolHandler
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("browser", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "browser"
}

func (t *Toolset) Version() string {
	return "0.1.0"
}

func (t *Toolset) Init(ctx mcp.ToolsetContext) error {
	t.ctx = ctx
	t.enabled = browserEnabled()
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	if !t.enabled {
		return nil
	}
	for _, tool := range t.tools() {
		spec := mcp.ToolSpec{
			Name:        tool.name,
			Description: tool.description,
			ToolsetID:   t.ID(),
			InputSchema: tool.schema,
			Safety:      tool.safety,
			Handler:     tool.handler,
		}
		if err := reg.Add(spec); err != nil {
			return fmt.Errorf("register %s: %w", tool.name, err)
		}
	}
	return nil
}

func browserEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("MCP_BROWSER_ENABLED")))
	switch value {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func schemaObject(required []string, properties map[string]any) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func withCommon(props map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range props {
		out[k] = v
	}
	out["session"] = map[string]any{"type": "string", "description": "Optional browser session id"}
	out["profile"] = map[string]any{"type": "string", "description": "Optional browser profile name"}
	out["state"] = map[string]any{"type": "string", "description": "Optional saved state path"}
	out["timeoutSeconds"] = map[string]any{"type": "number", "description": "Optional command timeout in seconds"}
	out["extraArgs"] = map[string]any{
		"type":        "array",
		"description": "Additional CLI args passed to agent-browser",
		"items":       map[string]any{"type": "string"},
	}
	return out
}

func (t *Toolset) tools() []toolDef {
	read := mcp.SafetyReadOnly
	write := mcp.SafetyRiskyWrite

	urlSchema := schemaObject([]string{"url"}, withCommon(map[string]any{"url": map[string]any{"type": "string"}}))
	selectorSchema := schemaObject([]string{"selector"}, withCommon(map[string]any{"selector": map[string]any{"type": "string"}}))

	return []toolDef{
		{name: "browser_open", description: "Open a URL in browser session.", safety: read, schema: urlSchema, handler: t.handleOpen},
		{name: "browser_screenshot", description: "Capture screenshot of current page.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{"path": map[string]any{"type": "string"}})), handler: t.handleScreenshot},
		{name: "browser_click", description: "Click an element by selector.", safety: write, schema: selectorSchema, handler: t.handleClick},
		{name: "browser_fill", description: "Fill input field with text.", safety: write, schema: schemaObject([]string{"selector", "text"}, withCommon(map[string]any{"selector": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}})), handler: t.handleFill},
		{name: "browser_test_ingress", description: "Open ingress URL and return basic status evidence.", safety: read, schema: urlSchema, handler: t.handleTestIngress},
		{name: "browser_screenshot_grafana", description: "Open Grafana URL and capture dashboard screenshot.", safety: read, schema: schemaObject([]string{"url"}, withCommon(map[string]any{"url": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}})), handler: t.handleScreenshotGrafana},
		{name: "browser_health_check", description: "Run web health check against application URL.", safety: read, schema: schemaObject([]string{"url"}, withCommon(map[string]any{"url": map[string]any{"type": "string"}, "contains": map[string]any{"type": "string"}})), handler: t.handleHealthCheck},
		{name: "browser_snapshot", description: "Capture page snapshot from current session.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{})), handler: t.handleSnapshot},
		{name: "browser_get_text", description: "Extract text from element selector.", safety: read, schema: selectorSchema, handler: t.handleGetText},
		{name: "browser_get_html", description: "Extract HTML from element selector.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{"selector": map[string]any{"type": "string"}})), handler: t.handleGetHTML},
		{name: "browser_evaluate", description: "Evaluate JavaScript expression in current page.", safety: read, schema: schemaObject([]string{"expression"}, withCommon(map[string]any{"expression": map[string]any{"type": "string"}})), handler: t.handleEvaluate},
		{name: "browser_pdf", description: "Export current page to PDF.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{"path": map[string]any{"type": "string"}})), handler: t.handlePDF},
		{name: "browser_wait_for", description: "Wait for selector to appear.", safety: read, schema: selectorSchema, handler: t.handleWaitFor},
		{name: "browser_wait_for_url", description: "Wait for URL to match expected value.", safety: read, schema: urlSchema, handler: t.handleWaitForURL},
		{name: "browser_press", description: "Press a keyboard key.", safety: write, schema: schemaObject([]string{"key"}, withCommon(map[string]any{"key": map[string]any{"type": "string"}})), handler: t.handlePress},
		{name: "browser_select", description: "Select option in dropdown.", safety: write, schema: schemaObject([]string{"selector", "value"}, withCommon(map[string]any{"selector": map[string]any{"type": "string"}, "value": map[string]any{"type": "string"}})), handler: t.handleSelect},
		{name: "browser_check", description: "Check checkbox or radio element.", safety: write, schema: selectorSchema, handler: t.handleCheck},
		{name: "browser_uncheck", description: "Uncheck checkbox element.", safety: write, schema: selectorSchema, handler: t.handleUncheck},
		{name: "browser_hover", description: "Hover over an element by selector.", safety: read, schema: selectorSchema, handler: t.handleHover},
		{name: "browser_type", description: "Type text into element by selector.", safety: write, schema: schemaObject([]string{"selector", "text"}, withCommon(map[string]any{"selector": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}})), handler: t.handleType},
		{name: "browser_upload", description: "Upload file using file input selector.", safety: write, schema: schemaObject([]string{"selector", "path"}, withCommon(map[string]any{"selector": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}})), handler: t.handleUpload},
		{name: "browser_drag", description: "Drag element from source to target selector.", safety: write, schema: schemaObject([]string{"source", "target"}, withCommon(map[string]any{"source": map[string]any{"type": "string"}, "target": map[string]any{"type": "string"}})), handler: t.handleDrag},
		{name: "browser_new_tab", description: "Open new tab, optionally with URL.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{"url": map[string]any{"type": "string"}})), handler: t.handleNewTab},
		{name: "browser_switch_tab", description: "Switch active browser tab.", safety: read, schema: schemaObject([]string{"tab"}, withCommon(map[string]any{"tab": map[string]any{"type": "string"}})), handler: t.handleSwitchTab},
		{name: "browser_close_tab", description: "Close browser tab.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{"tab": map[string]any{"type": "string"}})), handler: t.handleCloseTab},
		{name: "browser_close", description: "Close browser session.", safety: read, schema: schemaObject(nil, withCommon(map[string]any{})), handler: t.handleClose},
	}
}

func require(args map[string]any, key string) (string, error) {
	value := strings.TrimSpace(toString(args[key]))
	if value == "" {
		return "", fmt.Errorf("missing required field: %s", key)
	}
	return value, nil
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toInt(v any, fallback int) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return fallback
	}
}

func toStringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s := strings.TrimSpace(toString(item))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func ensureEnabled() error {
	if browserEnabled() {
		return nil
	}
	return errors.New("browser automation disabled: set MCP_BROWSER_ENABLED=true")
}
