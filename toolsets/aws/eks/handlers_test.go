package awseks

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	eksCalled := false
	ec2Called := false
	asgCalled := false
	svc := &Service{
		ctx: ctx,
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			eksCalled = true
			return nil, "", nil
		},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			ec2Called = true
			return nil, "", nil
		},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			asgCalled = true
			return nil, "", nil
		},
	}

	tests := []struct {
		name    string
		handler func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{"getClusterMissing", svc.handleGetCluster, map[string]any{}, "name is required"},
		{"listNodegroupsMissing", svc.handleListNodegroups, map[string]any{}, "clusterName is required"},
		{"getNodegroupMissing", svc.handleGetNodegroup, map[string]any{}, "clusterName and nodegroupName are required"},
		{"listAddonsMissing", svc.handleListAddons, map[string]any{}, "clusterName is required"},
		{"getAddonMissing", svc.handleGetAddon, map[string]any{}, "clusterName and addonName are required"},
		{"listFargateProfilesMissing", svc.handleListFargateProfiles, map[string]any{}, "clusterName is required"},
		{"getFargateProfileMissing", svc.handleGetFargateProfile, map[string]any{}, "clusterName and profileName are required"},
		{"listIdentityProvidersMissing", svc.handleListIdentityProviderConfigs, map[string]any{}, "clusterName is required"},
		{"getIdentityProviderMissing", svc.handleGetIdentityProviderConfig, map[string]any{}, "clusterName, type, and name are required"},
		{"listUpdatesMissing", svc.handleListUpdates, map[string]any{}, "clusterName is required"},
		{"getUpdateMissing", svc.handleGetUpdate, map[string]any{}, "clusterName and updateId are required"},
		{"listNodesMissing", svc.handleListNodes, map[string]any{}, "clusterName is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eksCalled = false
			ec2Called = false
			asgCalled = false
			_, err := tt.handler(context.Background(), mcp.ToolRequest{Arguments: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
			if eksCalled || ec2Called || asgCalled {
				t.Fatalf("client should not be invoked")
			}
		})
	}
}
