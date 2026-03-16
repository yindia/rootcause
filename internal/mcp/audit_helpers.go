package mcp

import (
	"context"
	"time"

	"rootcause/internal/audit"
)

func logAudit(callCtx context.Context, ctx ToolContext, spec ToolSpec, userID string, namespaces, resources []string, outcome string, err error) {
	if ctx.Audit == nil {
		return
	}
	traceID, _ := traceIDFromContext(callCtx)
	callChain, _ := callChainFromContext(callCtx)
	parentTool := ""
	if len(callChain) > 1 {
		parentTool = callChain[len(callChain)-2]
	}
	event := audit.Event{
		Timestamp:  time.Now().UTC(),
		UserID:     userID,
		TraceID:    traceID,
		ParentTool: parentTool,
		CallChain:  callChain,
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
