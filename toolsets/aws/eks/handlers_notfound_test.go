package awseks

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSGetNotFoundBranches(t *testing.T) {
	responses := map[string]string{
		"/clusters/demo":                                    `{}`,
		"/clusters/demo/addons/vpc-cni":                     `{}`,
		"/clusters/demo/node-groups/ng-1":                   `{}`,
		"/clusters/demo/fargate-profiles/fp-1":              `{}`,
		"/clusters/demo/identity-provider-configs/describe": `{}`,
		"/clusters/demo/updates/upd-1":                      `{}`,
	}
	client := newEKSTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleGetCluster(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"name": "demo"}}); err == nil {
		t.Fatalf("expected cluster not found")
	}
	if _, err := svc.handleGetAddon(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "addonName": "vpc-cni"}}); err == nil {
		t.Fatalf("expected addon not found")
	}
	if _, err := svc.handleGetNodegroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "nodegroupName": "ng-1"}}); err == nil {
		t.Fatalf("expected nodegroup not found")
	}
	if _, err := svc.handleGetFargateProfile(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "profileName": "fp-1"}}); err == nil {
		t.Fatalf("expected fargate profile not found")
	}
	if _, err := svc.handleGetIdentityProviderConfig(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "type": "oidc", "name": "idp"}}); err == nil {
		t.Fatalf("expected identity provider config not found")
	}
	if _, err := svc.handleGetUpdate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "updateId": "upd-1"}}); err == nil {
		t.Fatalf("expected update not found")
	}
}

func TestEKSTypeHelpers(t *testing.T) {
	if got := toStringSlice([]any{"a", 1}); len(got) != 1 || got[0] != "a" {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toInt(float64(5), 1); got != 5 {
		t.Fatalf("unexpected toInt: %d", got)
	}
}
