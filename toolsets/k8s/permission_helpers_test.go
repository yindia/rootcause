package k8s

import (
	"context"
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

func TestAssumePolicyMentionsServiceAccount(t *testing.T) {
	data := map[string]any{
		"assumeRolePolicy": map[string]any{
			"Statement": []any{map[string]any{"Principal": "system:serviceaccount:default:api"}},
		},
	}
	if !assumePolicyMentionsServiceAccount(data, "default", "api") {
		t.Fatalf("expected service account to be detected")
	}
	if assumePolicyMentionsServiceAccount(data, "default", "missing") {
		t.Fatalf("expected missing service account")
	}
	if assumePolicyMentionsServiceAccount(nil, "default", "api") {
		t.Fatalf("expected nil policy to be false")
	}
	if assumePolicyMentionsServiceAccount(map[string]any{}, "default", "api") {
		t.Fatalf("expected empty policy to be false")
	}
}

func TestAddAWSRoleEvidence(t *testing.T) {
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	ctx := mcp.ToolContext{Config: &cfg, Registry: reg, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()}
	invoker := mcp.NewToolInvoker(reg, ctx)
	ctx.Invoker = invoker
	toolset := New()
	_ = toolset.Init(mcp.ToolsetContext{Config: &cfg, Clients: &kube.Clients{}, Registry: reg, Invoker: invoker, Policy: policy.NewAuthorizer(), Renderer: render.NewRenderer(), Redactor: redact.New()})

	_ = reg.Add(mcp.ToolSpec{
		Name:      "aws.iam.get_role",
		ToolsetID: "aws",
		Handler: func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error) {
			return mcp.ToolResult{Data: map[string]any{"assumeRolePolicy": "system:serviceaccount:default:api"}}, nil
		},
	})

	analysis := render.NewAnalysis()
	toolset.addAWSRoleEvidence(context.Background(), mcp.ToolRequest{User: policy.User{Role: policy.RoleCluster}}, &analysis, "demo", "us-east-1", "default", "api")
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected aws role evidence")
	}
}
