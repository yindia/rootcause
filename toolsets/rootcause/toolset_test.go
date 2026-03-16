package rootcause

import (
	"context"
	"fmt"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
)

func TestToolsetInitAndRegister(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected init error without invoker")
	}
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("rootcause.incident_bundle"); !ok {
		t.Fatalf("expected rootcause.incident_bundle to be registered")
	}
	if _, ok := reg.Get("rootcause.rca_generate"); !ok {
		t.Fatalf("expected rootcause.rca_generate to be registered")
	}
	if _, ok := reg.Get("rootcause.remediation_playbook"); !ok {
		t.Fatalf("expected rootcause.remediation_playbook to be registered")
	}
	if _, ok := reg.Get("rootcause.postmortem_export"); !ok {
		t.Fatalf("expected rootcause.postmortem_export to be registered")
	}
	if _, ok := reg.Get("rootcause.change_timeline"); !ok {
		t.Fatalf("expected rootcause.change_timeline to be registered")
	}
	if _, ok := reg.Get("rootcause.capabilities"); !ok {
		t.Fatalf("expected rootcause.capabilities to be registered")
	}
}

func TestHandleIncidentBundleAggregatesSections(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}

	addFakeTool(t, reg, "k8s.overview")
	addFakeTool(t, reg, "k8s.events_timeline")
	addFakeTool(t, reg, "k8s.diagnose")
	addFakeTool(t, reg, "helm.list")

	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := toolset.handleIncidentBundle(context.Background(), mcp.ToolRequest{
		User: policy.User{ID: "u1", Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}},
		Arguments: map[string]any{
			"namespace": "default",
			"keyword":   "api",
		},
	})
	if err != nil {
		t.Fatalf("handleIncidentBundle: %v", err)
	}
	root := result.Data.(map[string]any)
	if got := intOrDefault(root["sectionCount"], 0); got < 3 {
		t.Fatalf("expected at least 3 sections, got %d", got)
	}
	if got := intOrDefault(root["errorCount"], 0); got != 0 {
		t.Fatalf("expected no errors, got %d", got)
	}
	if got := intOrDefault(root["stepCount"], 0); got < 3 {
		t.Fatalf("expected at least 3 steps, got %d", got)
	}
}

func TestHandleIncidentBundleCustomChain(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	addFakeTool(t, reg, "k8s.overview")
	addFakeTool(t, reg, "k8s.events_timeline")
	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := toolset.handleIncidentBundle(context.Background(), mcp.ToolRequest{
		User: policy.User{ID: "u1", Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}},
		Arguments: map[string]any{
			"includeDefaultChain": false,
			"toolChain": []any{
				map[string]any{"tool": "k8s.overview", "section": "a", "args": map[string]any{"namespace": "default"}},
				map[string]any{"tool": "k8s.events_timeline", "section": "b", "args": map[string]any{"namespace": "default", "limit": 10}},
			},
		},
	})
	if err != nil {
		t.Fatalf("handleIncidentBundle: %v", err)
	}
	root := result.Data.(map[string]any)
	sections := root["sections"].(map[string]any)
	if _, ok := sections["a"]; !ok {
		t.Fatalf("expected custom section a")
	}
	if _, ok := sections["b"]; !ok {
		t.Fatalf("expected custom section b")
	}
}

func TestHandleIncidentBundleStopOnError(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	addFakeTool(t, reg, "k8s.overview")
	_ = reg.Add(mcp.ToolSpec{
		Name:      "fake.fail",
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"error": "fail"}}, fmt.Errorf("fail")
		},
	})
	addFakeTool(t, reg, "k8s.events_timeline")
	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := toolset.handleIncidentBundle(context.Background(), mcp.ToolRequest{
		User: policy.User{ID: "u1", Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}},
		Arguments: map[string]any{
			"includeDefaultChain": false,
			"continueOnError":     false,
			"toolChain": []any{
				map[string]any{"tool": "k8s.overview", "section": "a"},
				map[string]any{"tool": "fake.fail", "section": "fail"},
				map[string]any{"tool": "k8s.events_timeline", "section": "c"},
			},
		},
	})
	if err != nil {
		t.Fatalf("handleIncidentBundle should not hard-fail: %v", err)
	}
	root := result.Data.(map[string]any)
	if got := intOrDefault(root["errorCount"], 0); got == 0 {
		t.Fatalf("expected at least one error")
	}
	if got := intOrDefault(root["stepCount"], 0); got != 2 {
		t.Fatalf("expected chain stop after failure with 2 executed steps, got %d", got)
	}
}

func TestHandleRCAGenerateFromBundle(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{Invoker: &mcp.ToolInvoker{}}); err != nil {
		cfg := config.DefaultConfig()
		reg := mcp.NewRegistry(&cfg)
		ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
		ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
		if err := toolset.Init(ctx); err != nil {
			t.Fatalf("init: %v", err)
		}
	}
	bundle := map[string]any{
		"generatedAt": "2026-03-15T00:00:00Z",
		"errorCount":  0,
		"sections": map[string]any{
			"diagnose": map[string]any{
				"likelyRootCauses": []any{map[string]any{"summary": "CrashLoopBackOff from bad config"}},
			},
		},
	}
	result, err := toolset.handleRCAGenerate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"bundle": bundle}})
	if err != nil {
		t.Fatalf("handleRCAGenerate: %v", err)
	}
	root := result.Data.(map[string]any)
	rca := root["rca"].(map[string]any)
	if toString(rca["confidence"]) != "high" {
		t.Fatalf("expected high confidence, got %v", rca["confidence"])
	}
}

func TestHandleRemediationPlaybookFromInputs(t *testing.T) {
	toolset := New()
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	bundle := map[string]any{"generatedAt": "2026-03-15T00:00:00Z", "namespace": "default", "sections": map[string]any{}, "errorCount": 0}
	rca := map[string]any{"rootCauses": []any{"Config drift"}, "recommendations": []any{"Pin values file"}, "confidence": "high"}
	result, err := toolset.handleRemediationPlaybook(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}, Arguments: map[string]any{"bundle": bundle, "rca": rca}})
	if err != nil {
		t.Fatalf("handleRemediationPlaybook: %v", err)
	}
	root := result.Data.(map[string]any)
	playbook := root["playbook"].(map[string]any)
	immediate := playbook["immediateActions"].([]map[string]any)
	if len(immediate) == 0 {
		t.Fatalf("expected immediate actions")
	}
}

func TestHandlePostmortemExportMarkdown(t *testing.T) {
	toolset := New()
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	bundle := map[string]any{"generatedAt": "2026-03-15T00:00:00Z", "sections": map[string]any{}, "errorCount": 0}
	rca := map[string]any{"incidentSummary": "outage", "rootCauses": []any{"CrashLoop"}, "recommendations": []any{"Fix config"}, "confidence": "high"}
	result, err := toolset.handlePostmortemExport(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}, Arguments: map[string]any{"bundle": bundle, "rca": rca, "format": "markdown"}})
	if err != nil {
		t.Fatalf("handlePostmortemExport: %v", err)
	}
	root := result.Data.(map[string]any)
	if toString(root["format"]) != "markdown" {
		t.Fatalf("expected markdown format")
	}
	if content := toString(root["content"]); content == "" {
		t.Fatalf("expected markdown content")
	}
}

func TestHandleChangeTimeline(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	_ = reg.Add(mcp.ToolSpec{
		Name:      "k8s.events_timeline",
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"timeline": []any{
				map[string]any{"time": "2026-03-15T00:00:02Z", "type": "Warning", "reason": "BackOff", "message": "back off", "object": map[string]any{"kind": "Pod", "name": "api-1"}},
				map[string]any{"time": "2026-03-15T00:00:01Z", "type": "Warning", "reason": "FailedScheduling", "message": "no nodes", "object": map[string]any{"kind": "Pod", "name": "api-2"}},
			}}}, nil
		},
	})
	_ = reg.Add(mcp.ToolSpec{
		Name:      "helm.list",
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"releases": []any{map[string]any{"name": "payment-api", "status": "failed", "revision": 12, "updated": "2026-03-15T00:00:03Z"}}}}, nil
		},
	})

	ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	result, err := toolset.handleChangeTimeline(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}}, Arguments: map[string]any{"namespace": "payments", "includeHelm": true, "timelineLimit": 10}})
	if err != nil {
		t.Fatalf("handleChangeTimeline: %v", err)
	}
	root := result.Data.(map[string]any)
	if got := intOrDefault(root["timelineCount"], 0); got != 3 {
		t.Fatalf("expected timelineCount=3, got %d", got)
	}
	timeline := root["timeline"].([]map[string]any)
	if toString(timeline[0]["time"]) != "2026-03-15T00:00:01Z" {
		t.Fatalf("expected sorted timeline, got first=%v", timeline[0]["time"])
	}
}

func TestHandleCapabilities(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	result, err := toolset.handleCapabilities(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"includeSchemas": true}})
	if err != nil {
		t.Fatalf("handleCapabilities: %v", err)
	}
	root, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response")
	}
	if _, ok := root["dependencyGraph"].(map[string]any); !ok {
		t.Fatalf("expected dependencyGraph in response")
	}
}

func TestHandleIncidentBundleTimelineMode(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg}
	_ = reg.Add(mcp.ToolSpec{
		Name:      "k8s.events_timeline",
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"timeline": []any{map[string]any{"time": "2026-03-15T00:00:01Z", "type": "Warning", "reason": "BackOff", "message": "back off", "object": map[string]any{"kind": "Pod", "name": "api-1"}}}}}, nil
		},
	})
	_ = reg.Add(mcp.ToolSpec{
		Name:      "k8s.overview",
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"ok": true}}, nil
		},
	})
	ctx.Invoker = mcp.NewToolInvoker(reg, ctx)
	toolset := New()
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	result, err := toolset.handleIncidentBundle(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster, AllowedNamespaces: []string{"*"}}, Arguments: map[string]any{"namespace": "payments", "outputMode": "timeline", "includeHelm": false, "keyword": "backoff"}})
	if err != nil {
		t.Fatalf("handleIncidentBundle timeline mode: %v", err)
	}
	root := result.Data.(map[string]any)
	if got := intOrDefault(root["timelineCount"], 0); got != 1 {
		t.Fatalf("expected timelineCount=1, got %d", got)
	}
}

func addFakeTool(t *testing.T, reg mcp.Registry, name string) {
	t.Helper()
	err := reg.Add(mcp.ToolSpec{
		Name:      name,
		ToolsetID: "fake",
		Safety:    mcp.SafetyReadOnly,
		Handler: func(ctx context.Context, req mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"tool": name, "args": req.Arguments}}, nil
		},
	})
	if err != nil {
		t.Fatalf("add fake tool %s: %v", name, err)
	}
}
