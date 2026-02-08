package awsec2

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2HandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	ec2Called := false
	asgCalled := false
	elbCalled := false
	iamCalled := false
	svc := &Service{
		ctx: ctx,
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			ec2Called = true
			return nil, "", nil
		},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			asgCalled = true
			return nil, "", nil
		},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			elbCalled = true
			return nil, "", nil
		},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			iamCalled = true
			return nil, "", nil
		},
	}

	tests := []struct {
		name    string
		handler func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{"getInstanceMissing", svc.handleGetInstance, map[string]any{}, "instanceId is required"},
		{"getASGMissing", svc.handleGetASG, map[string]any{}, "autoScalingGroupName is required"},
		{"getLoadBalancerMissing", svc.handleGetLoadBalancer, map[string]any{}, "loadBalancerArn or name is required"},
		{"getTargetGroupMissing", svc.handleGetTargetGroup, map[string]any{}, "targetGroupArn or name is required"},
		{"getListenerMissing", svc.handleGetListener, map[string]any{}, "listenerArn is required"},
		{"getTargetHealthMissing", svc.handleGetTargetHealth, map[string]any{}, "targetGroupArn is required"},
		{"listListenerRulesMissing", svc.handleListListenerRules, map[string]any{}, "listenerArn or ruleArns is required"},
		{"getListenerRuleMissing", svc.handleGetListenerRule, map[string]any{}, "ruleArn is required"},
		{"getScalingPolicyMissing", svc.handleGetAutoScalingPolicy, map[string]any{}, "policyName or autoScalingGroupName is required"},
		{"getScalingActivityMissing", svc.handleGetScalingActivity, map[string]any{}, "activityId is required"},
		{"getLaunchTemplateMissing", svc.handleGetLaunchTemplate, map[string]any{}, "launchTemplateId or name is required"},
		{"getLaunchConfigMissing", svc.handleGetLaunchConfiguration, map[string]any{}, "launchConfigurationName is required"},
		{"getInstanceIAMMissing", svc.handleGetInstanceIAM, map[string]any{}, "instanceId is required"},
		{"getSecurityGroupRulesMissing", svc.handleGetSecurityGroupRules, map[string]any{}, "groupId is required"},
		{"getSpotRequestMissing", svc.handleGetSpotInstanceRequest, map[string]any{}, "spotInstanceRequestId is required"},
		{"getCapacityReservationMissing", svc.handleGetCapacityReservation, map[string]any{}, "capacityReservationId is required"},
		{"getVolumeMissing", svc.handleGetVolume, map[string]any{}, "volumeId is required"},
		{"getSnapshotMissing", svc.handleGetSnapshot, map[string]any{}, "snapshotId is required"},
		{"getPlacementGroupMissing", svc.handleGetPlacementGroup, map[string]any{}, "groupName is required"},
		{"getInstanceStatusMissing", svc.handleGetInstanceStatus, map[string]any{}, "instanceId is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec2Called = false
			asgCalled = false
			elbCalled = false
			iamCalled = false
			_, err := tt.handler(context.Background(), mcp.ToolRequest{Arguments: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
			if ec2Called || asgCalled || elbCalled || iamCalled {
				t.Fatalf("client should not be invoked")
			}
		})
	}
}
