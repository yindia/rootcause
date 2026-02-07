package awsec2

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2ListFiltersMore(t *testing.T) {
	ec2Responses := map[string]string{
		"DescribeLaunchConfigurations": `<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations>
      <member>
        <LaunchConfigurationName>lc-1</LaunchConfigurationName>
        <ImageId>ami-1</ImageId>
        <InstanceType>t3.micro</InstanceType>
      </member>
    </LaunchConfigurations>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
		"DescribeCapacityReservations": `<DescribeCapacityReservationsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <capacityReservationSet>
    <item>
      <capacityReservationId>cr-1</capacityReservationId>
      <instanceType>t3.micro</instanceType>
      <availabilityZone>us-east-1a</availabilityZone>
      <totalInstanceCount>1</totalInstanceCount>
      <availableInstanceCount>1</availableInstanceCount>
      <state>active</state>
    </item>
  </capacityReservationSet>
</DescribeCapacityReservationsResponse>`,
		"DescribeInstanceStatus": `<DescribeInstanceStatusResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <instanceStatusSet>
    <item>
      <instanceId>i-1</instanceId>
      <availabilityZone>us-east-1a</availabilityZone>
      <instanceState><name>running</name></instanceState>
      <systemStatus><status>ok</status></systemStatus>
      <instanceStatus><status>ok</status></instanceStatus>
    </item>
  </instanceStatusSet>
</DescribeInstanceStatusResponse>`,
		"DescribeVolumes": `<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet>
    <item>
      <volumeId>vol-1</volumeId>
      <availabilityZone>us-east-1a</availabilityZone>
      <size>10</size>
      <status>in-use</status>
      <volumeType>gp3</volumeType>
      <attachmentSet>
        <item>
          <instanceId>i-1</instanceId>
          <state>attached</state>
          <device>/dev/xvda</device>
        </item>
      </attachmentSet>
    </item>
  </volumeSet>
</DescribeVolumesResponse>`,
	}
	asgResponses := map[string]string{
		"DescribeLaunchConfigurations": `<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations>
      <member>
        <LaunchConfigurationName>lc-1</LaunchConfigurationName>
        <ImageId>ami-1</ImageId>
        <InstanceType>t3.micro</InstanceType>
      </member>
    </LaunchConfigurations>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
		"DescribePolicies": `<DescribePoliciesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribePoliciesResult>
    <ScalingPolicies>
      <member>
        <PolicyName>policy-1</PolicyName>
        <PolicyARN>arn:policy</PolicyARN>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <PolicyType>TargetTrackingScaling</PolicyType>
      </member>
    </ScalingPolicies>
  </DescribePoliciesResult>
</DescribePoliciesResponse>`,
	}
	elbResponses := map[string]string{
		"DescribeTargetGroups": `<DescribeTargetGroupsResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeTargetGroupsResult>
    <TargetGroups>
      <member>
        <TargetGroupArn>arn:tg</TargetGroupArn>
        <TargetGroupName>tg-1</TargetGroupName>
        <Port>80</Port>
        <Protocol>HTTP</Protocol>
        <VpcId>vpc-1</VpcId>
      </member>
    </TargetGroups>
  </DescribeTargetGroupsResult>
</DescribeTargetGroupsResponse>`,
		"DescribeListeners": `<DescribeListenersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeListenersResult>
    <Listeners>
      <member>
        <ListenerArn>arn:listener</ListenerArn>
        <LoadBalancerArn>arn:lb</LoadBalancerArn>
        <Port>443</Port>
        <Protocol>HTTPS</Protocol>
      </member>
    </Listeners>
  </DescribeListenersResult>
</DescribeListenersResponse>`,
	}

	ec2Client := newEC2TestClient(t, ec2Responses)
	asgClient := newASGTestClient(t, asgResponses)
	elbClient := newELBTestClient(t, elbResponses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2Client, "us-east-1", nil
		},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return asgClient, "us-east-1", nil
		},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return elbClient, "us-east-1", nil
		},
	}

	if _, err := svc.handleListLaunchConfigurations(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"launchConfigurationNames": []string{"lc-1"},
		"limit":                    1,
	}}); err != nil {
		t.Fatalf("list launch configurations with names: %v", err)
	}
	if _, err := svc.handleListCapacityReservations(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"capacityReservationIds": []string{"cr-1"},
		"limit":                  1,
	}}); err != nil {
		t.Fatalf("list capacity reservations with ids: %v", err)
	}
	if _, err := svc.handleListInstanceStatus(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"instanceIds": []string{"i-1"},
		"includeAll":  false,
		"limit":       1,
	}}); err != nil {
		t.Fatalf("list instance status with ids: %v", err)
	}
	if _, err := svc.handleListVolumeAttachments(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"volumeId":   "vol-1",
		"instanceId": "i-1",
		"limit":      1,
	}}); err != nil {
		t.Fatalf("list volume attachments with filters: %v", err)
	}
	if _, err := svc.handleListTargetGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"targetGroupArns": []string{"arn:tg"},
		"loadBalancerArn": "arn:lb",
		"limit":           1,
	}}); err != nil {
		t.Fatalf("list target groups with filters: %v", err)
	}
	if _, err := svc.handleListListeners(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"listenerArns":   []string{"arn:listener"},
		"loadBalancerArn": "arn:lb",
		"limit":          1,
	}}); err != nil {
		t.Fatalf("list listeners with filters: %v", err)
	}
	if _, err := svc.handleListAutoScalingPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyNames":         []string{"policy-1"},
		"autoScalingGroupName": "asg-1",
		"limit":               1,
	}}); err != nil {
		t.Fatalf("list autoscaling policies with filters: %v", err)
	}
}
