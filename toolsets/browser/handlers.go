package browser

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"rootcause/internal/mcp"
)

func (t *Toolset) handleOpen(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url, err := require(req.Arguments, "url")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"open", url})
}

func (t *Toolset) handleScreenshot(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	path := strings.TrimSpace(toString(req.Arguments["path"]))
	base := []string{"screenshot"}
	if path != "" {
		base = append(base, path)
	}
	return t.runCommand(ctx, req.Arguments, base)
}

func (t *Toolset) handleClick(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"click", selector})
}

func (t *Toolset) handleFill(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	text, err := require(req.Arguments, "text")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"fill", selector, text})
}

func (t *Toolset) handleSnapshot(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.runCommand(ctx, req.Arguments, []string{"snapshot"})
}

func (t *Toolset) handleGetText(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"get", "text", selector})
}

func (t *Toolset) handleGetHTML(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector := strings.TrimSpace(toString(req.Arguments["selector"]))
	base := []string{"get", "html"}
	if selector != "" {
		base = append(base, selector)
	}
	return t.runCommand(ctx, req.Arguments, base)
}

func (t *Toolset) handleEvaluate(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	expression, err := require(req.Arguments, "expression")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"evaluate", expression})
}

func (t *Toolset) handlePDF(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	path := strings.TrimSpace(toString(req.Arguments["path"]))
	base := []string{"pdf"}
	if path != "" {
		base = append(base, path)
	}
	return t.runCommand(ctx, req.Arguments, base)
}

func (t *Toolset) handleWaitFor(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"wait", "for", selector})
}

func (t *Toolset) handleWaitForURL(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url, err := require(req.Arguments, "url")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"wait", "for-url", url})
}

func (t *Toolset) handlePress(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	key, err := require(req.Arguments, "key")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"press", key})
}

func (t *Toolset) handleSelect(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	value, err := require(req.Arguments, "value")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"select", selector, value})
}

func (t *Toolset) handleCheck(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"check", selector})
}

func (t *Toolset) handleUncheck(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"uncheck", selector})
}

func (t *Toolset) handleHover(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"hover", selector})
}

func (t *Toolset) handleType(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	text, err := require(req.Arguments, "text")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"type", selector, text})
}

func (t *Toolset) handleUpload(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	selector, err := require(req.Arguments, "selector")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	path, err := require(req.Arguments, "path")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"upload", selector, path})
}

func (t *Toolset) handleDrag(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	source, err := require(req.Arguments, "source")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	target, err := require(req.Arguments, "target")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"drag", source, target})
}

func (t *Toolset) handleNewTab(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url := strings.TrimSpace(toString(req.Arguments["url"]))
	base := []string{"new", "tab"}
	if url != "" {
		base = append(base, url)
	}
	return t.runCommand(ctx, req.Arguments, base)
}

func (t *Toolset) handleSwitchTab(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	tab, err := require(req.Arguments, "tab")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"switch", "tab", tab})
}

func (t *Toolset) handleCloseTab(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	tab := strings.TrimSpace(toString(req.Arguments["tab"]))
	base := []string{"close", "tab"}
	if tab != "" {
		base = append(base, tab)
	}
	return t.runCommand(ctx, req.Arguments, base)
}

func (t *Toolset) handleClose(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	return t.runCommand(ctx, req.Arguments, []string{"close"})
}

func (t *Toolset) handleTestIngress(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url, err := require(req.Arguments, "url")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	result, err := t.runCommand(ctx, req.Arguments, []string{"open", url})
	if err != nil {
		return result, err
	}
	snap, snapErr := t.runCommand(ctx, req.Arguments, []string{"snapshot"})
	if snapErr != nil {
		return mcp.ToolResult{Data: map[string]any{"open": result.Data, "snapshot_error": snapErr.Error()}}, nil
	}
	return mcp.ToolResult{Data: map[string]any{"open": result.Data, "snapshot": snap.Data}}, nil
}

func (t *Toolset) handleScreenshotGrafana(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url, err := require(req.Arguments, "url")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	path := strings.TrimSpace(toString(req.Arguments["path"]))
	if path == "" {
		path = "grafana-dashboard.png"
	}
	if _, err := t.runCommand(ctx, req.Arguments, []string{"open", url}); err != nil {
		return mcp.ToolResult{}, err
	}
	return t.runCommand(ctx, req.Arguments, []string{"screenshot", path})
}

func (t *Toolset) handleHealthCheck(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
	url, err := require(req.Arguments, "url")
	if err != nil {
		return mcp.ToolResult{}, err
	}
	timeout := toInt(req.Arguments["timeoutSeconds"], 15)
	callCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}
	request, err := http.NewRequestWithContext(callCtx, http.MethodGet, url, nil)
	if err != nil {
		return mcp.ToolResult{}, err
	}
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return mcp.ToolResult{}, err
	}
	defer resp.Body.Close()
	contains := strings.TrimSpace(toString(req.Arguments["contains"]))
	statusOK := resp.StatusCode >= 200 && resp.StatusCode < 400
	containsOK := true
	if contains != "" {
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		containsOK = strings.Contains(string(buf[:n]), contains)
	}
	if !statusOK || !containsOK {
		return mcp.ToolResult{Data: map[string]any{"url": url, "statusCode": resp.StatusCode, "containsMatch": containsOK}}, fmt.Errorf("health check failed")
	}
	return mcp.ToolResult{Data: map[string]any{"url": url, "statusCode": resp.StatusCode, "healthy": true}}, nil
}

func (t *Toolset) runCommand(ctx context.Context, arguments map[string]any, base []string) (mcp.ToolResult, error) {
	if err := ensureEnabled(); err != nil {
		return mcp.ToolResult{}, err
	}
	args := append([]string{}, base...)
	args = append([]string{"--json", "--max-output", "50000"}, args...)
	if session := strings.TrimSpace(toString(arguments["session"])); session != "" {
		args = append(args, "--session-name", session)
	}
	if profile := strings.TrimSpace(toString(arguments["profile"])); profile != "" {
		args = append(args, "--profile", profile)
	}
	if state := strings.TrimSpace(toString(arguments["state"])); state != "" {
		args = append(args, "--state", state)
	}
	args = append(args, toStringSlice(arguments["extraArgs"])...)

	callCtx := ctx
	timeout := toInt(arguments["timeoutSeconds"], 0)
	if timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	stdout, stderr, err := runAgentBrowser(callCtx, args)
	result := mcp.ToolResult{Data: map[string]any{
		"command": strings.Join(append([]string{"agent-browser"}, args...), " "),
		"stdout":  strings.TrimSpace(stdout),
		"stderr":  strings.TrimSpace(stderr),
	}}
	if err != nil {
		return result, err
	}
	return result, nil
}

func runAgentBrowser(ctx context.Context, args []string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "agent-browser", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return stdout.String(), stderr.String(), fmt.Errorf("agent-browser not found; install with: npm install -g agent-browser && agent-browser install")
		}
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}
