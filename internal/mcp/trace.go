package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type traceContextKey string

const (
	traceIDKey   traceContextKey = "rootcause.trace_id"
	callChainKey traceContextKey = "rootcause.call_chain"
)

func ensureTraceContext(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := traceIDFromContext(ctx); !ok {
		ctx = context.WithValue(ctx, traceIDKey, newTraceID())
	}
	if _, ok := callChainFromContext(ctx); !ok {
		ctx = context.WithValue(ctx, callChainKey, []string{})
	}
	return ctx
}

func withTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ensureTraceContext(ctx)
	}
	ctx = ensureTraceContext(ctx)
	return context.WithValue(ctx, traceIDKey, traceID)
}

func withCallChain(ctx context.Context, chain []string) context.Context {
	ctx = ensureTraceContext(ctx)
	copyChain := append([]string{}, chain...)
	return context.WithValue(ctx, callChainKey, copyChain)
}

func traceIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	traceID, ok := ctx.Value(traceIDKey).(string)
	if !ok || traceID == "" {
		return "", false
	}
	return traceID, true
}

func callChainFromContext(ctx context.Context) ([]string, bool) {
	if ctx == nil {
		return nil, false
	}
	chain, ok := ctx.Value(callChainKey).([]string)
	if !ok {
		return nil, false
	}
	return append([]string{}, chain...), true
}

func newTraceID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "trace-fallback"
	}
	return hex.EncodeToString(buf)
}
