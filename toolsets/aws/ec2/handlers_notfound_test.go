package awsec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2GetNotFoundBranches(t *testing.T) {
	ec2Responses := map[string]string{
		"DescribeInstances": `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet></reservationSet>
</DescribeInstancesResponse>`,
		"DescribeLaunchTemplates": `<DescribeLaunchTemplatesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <launchTemplates></launchTemplates>
</DescribeLaunchTemplatesResponse>`,
		"DescribeSpotInstanceRequests": `<DescribeSpotInstanceRequestsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <spotInstanceRequestSet></spotInstanceRequestSet>
</DescribeSpotInstanceRequestsResponse>`,
		"DescribeCapacityReservations": `<DescribeCapacityReservationsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <capacityReservationSet></capacityReservationSet>
</DescribeCapacityReservationsResponse>`,
		"DescribeVolumes": `<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet></volumeSet>
</DescribeVolumesResponse>`,
		"DescribeSnapshots": `<DescribeSnapshotsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <snapshotSet></snapshotSet>
</DescribeSnapshotsResponse>`,
		"DescribePlacementGroups": `<DescribePlacementGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <placementGroupSet></placementGroupSet>
</DescribePlacementGroupsResponse>`,
		"DescribeInstanceStatus": `<DescribeInstanceStatusResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <instanceStatusSet></instanceStatusSet>
</DescribeInstanceStatusResponse>`,
		"DescribeSecurityGroups": `<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo></securityGroupInfo>
</DescribeSecurityGroupsResponse>`,
	}
	ec2Client := newEC2TestClient(t, ec2Responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2Client, "us-east-1", nil
		},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return nil, "us-east-1", nil
		},
	}
	if _, err := svc.handleGetInstance(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}}); err == nil {
		t.Fatalf("expected instance not found")
	}
	if _, err := svc.handleGetLaunchTemplate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"launchTemplateId": "lt-1"}}); err == nil {
		t.Fatalf("expected launch template not found")
	}
	if _, err := svc.handleGetSpotInstanceRequest(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"spotInstanceRequestId": "sir-1"}}); err == nil {
		t.Fatalf("expected spot request not found")
	}
	if _, err := svc.handleGetCapacityReservation(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"capacityReservationId": "cr-1"}}); err == nil {
		t.Fatalf("expected capacity reservation not found")
	}
	if _, err := svc.handleGetVolume(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"volumeId": "vol-1"}}); err == nil {
		t.Fatalf("expected volume not found")
	}
	if _, err := svc.handleGetSnapshot(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"snapshotId": "snap-1"}}); err == nil {
		t.Fatalf("expected snapshot not found")
	}
	if _, err := svc.handleGetPlacementGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupName": "pg-1"}}); err == nil {
		t.Fatalf("expected placement group not found")
	}
	if _, err := svc.handleGetInstanceStatus(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}}); err == nil {
		t.Fatalf("expected instance status not found")
	}
	if _, err := svc.handleGetSecurityGroupRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupId": "sg-1"}}); err == nil {
		t.Fatalf("expected security group not found")
	}
}

func TestEC2GetNotFoundASGAndELB(t *testing.T) {
	asgResponses := map[string]string{
		"DescribeAutoScalingGroups": `<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups></AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`,
		"DescribePolicies": `<DescribePoliciesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribePoliciesResult>
    <ScalingPolicies></ScalingPolicies>
  </DescribePoliciesResult>
</DescribePoliciesResponse>`,
		"DescribeScalingActivities": `<DescribeScalingActivitiesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeScalingActivitiesResult>
    <Activities></Activities>
  </DescribeScalingActivitiesResult>
</DescribeScalingActivitiesResponse>`,
		"DescribeLaunchConfigurations": `<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations></LaunchConfigurations>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
	}
	elbResponses := map[string]string{
		"DescribeLoadBalancers": `<DescribeLoadBalancersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeLoadBalancersResult>
    <LoadBalancers></LoadBalancers>
  </DescribeLoadBalancersResult>
</DescribeLoadBalancersResponse>`,
		"DescribeTargetGroups": `<DescribeTargetGroupsResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeTargetGroupsResult>
    <TargetGroups></TargetGroups>
  </DescribeTargetGroupsResult>
</DescribeTargetGroupsResponse>`,
		"DescribeListeners": `<DescribeListenersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeListenersResult>
    <Listeners></Listeners>
  </DescribeListenersResult>
</DescribeListenersResponse>`,
		"DescribeTargetHealth": `<DescribeTargetHealthResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeTargetHealthResult>
    <TargetHealthDescriptions></TargetHealthDescriptions>
  </DescribeTargetHealthResult>
</DescribeTargetHealthResponse>`,
		"DescribeRules": `<DescribeRulesResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeRulesResult>
    <Rules></Rules>
  </DescribeRulesResult>
</DescribeRulesResponse>`,
	}

	asgClient := newASGTestClient(t, asgResponses)
	elbClient := newELBTestClient(t, elbResponses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return asgClient, "us-east-1", nil
		},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return elbClient, "us-east-1", nil
		},
	}

	if _, err := svc.handleGetASG(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"autoScalingGroupName": "asg-1"}}); err == nil {
		t.Fatalf("expected asg not found")
	}
	if _, err := svc.handleGetAutoScalingPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"policyName": "policy-1"}}); err == nil {
		t.Fatalf("expected scaling policy not found")
	}
	if _, err := svc.handleGetScalingActivity(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"activityId": "act-1"}}); err == nil {
		t.Fatalf("expected scaling activity not found")
	}
	if _, err := svc.handleGetLaunchConfiguration(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"launchConfigurationName": "lc-1"}}); err == nil {
		t.Fatalf("expected launch configuration not found")
	}
	if _, err := svc.handleGetLoadBalancer(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"loadBalancerArn": "arn:lb"}}); err == nil {
		t.Fatalf("expected load balancer not found")
	}
	if _, err := svc.handleGetTargetGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"targetGroupArn": "arn:tg"}}); err == nil {
		t.Fatalf("expected target group not found")
	}
	if _, err := svc.handleGetListener(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"listenerArn": "arn:listener"}}); err == nil {
		t.Fatalf("expected listener not found")
	}
	if _, err := svc.handleGetTargetHealth(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"targetGroupArn": "arn:tg"}}); err != nil {
		t.Fatalf("unexpected target health error: %v", err)
	}
	if _, err := svc.handleGetListenerRule(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"ruleArn": "arn:rule"}}); err == nil {
		t.Fatalf("expected listener rule not found")
	}
}
