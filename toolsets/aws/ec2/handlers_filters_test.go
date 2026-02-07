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

func TestEC2ListHandlersWithFilters(t *testing.T) {
	responses := map[string]string{
		"DescribeInstances": `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-1</instanceId>
          <instanceType>t3.micro</instanceType>
          <placement><availabilityZone>us-east-1a</availabilityZone></placement>
          <privateIpAddress>10.0.0.1</privateIpAddress>
          <subnetId>subnet-1</subnetId>
          <vpcId>vpc-1</vpcId>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`,
		"DescribeSpotInstanceRequests": `<DescribeSpotInstanceRequestsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <spotInstanceRequestSet>
    <item>
      <spotInstanceRequestId>sir-1</spotInstanceRequestId>
      <state>active</state>
    </item>
  </spotInstanceRequestSet>
</DescribeSpotInstanceRequestsResponse>`,
		"DescribeVolumes": `<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet>
    <item>
      <volumeId>vol-1</volumeId>
      <availabilityZone>us-east-1a</availabilityZone>
      <size>10</size>
      <status>in-use</status>
      <volumeType>gp3</volumeType>
    </item>
  </volumeSet>
</DescribeVolumesResponse>`,
		"DescribeSnapshots": `<DescribeSnapshotsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <snapshotSet>
    <item>
      <snapshotId>snap-1</snapshotId>
      <volumeId>vol-1</volumeId>
      <status>completed</status>
      <volumeSize>10</volumeSize>
    </item>
  </snapshotSet>
</DescribeSnapshotsResponse>`,
		"DescribeLaunchTemplates": `<DescribeLaunchTemplatesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <launchTemplates>
    <item>
      <launchTemplateId>lt-1</launchTemplateId>
      <launchTemplateName>tmpl</launchTemplateName>
      <createTime>2024-01-01T00:00:00Z</createTime>
      <createdBy>me</createdBy>
      <defaultVersionNumber>1</defaultVersionNumber>
      <latestVersionNumber>1</latestVersionNumber>
    </item>
  </launchTemplates>
</DescribeLaunchTemplatesResponse>`,
	}
	client := newEC2TestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListInstances(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"instanceIds": []string{"i-1"},
		"vpcId":       "vpc-1",
		"subnetId":    "subnet-1",
		"state":       "running",
		"limit":       1,
	}}); err != nil {
		t.Fatalf("list instances with filters: %v", err)
	}
	if _, err := svc.handleListSpotInstanceRequests(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"spotInstanceRequestIds": []string{"sir-1"},
		"states":                 []string{"active"},
		"limit":                  1,
	}}); err != nil {
		t.Fatalf("list spot requests with filters: %v", err)
	}
	if _, err := svc.handleListVolumes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"volumeIds": []string{"vol-1"},
		"instanceId": "i-1",
		"limit":      1,
	}}); err != nil {
		t.Fatalf("list volumes with filters: %v", err)
	}
	if _, err := svc.handleListSnapshots(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"snapshotIds": []string{"snap-1"},
		"ownerIds":    []string{"self"},
		"volumeId":    "vol-1",
		"limit":       1,
	}}); err != nil {
		t.Fatalf("list snapshots with filters: %v", err)
	}
	if _, err := svc.handleListLaunchTemplates(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"launchTemplateIds": []string{"lt-1"},
		"names":             []string{"tmpl"},
		"limit":             1,
	}}); err != nil {
		t.Fatalf("list launch templates with filters: %v", err)
	}
	if _, err := svc.handleGetLaunchTemplate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"name": "tmpl"}}); err != nil {
		t.Fatalf("get launch template by name: %v", err)
	}
}

func TestEC2AutoScalingFilters(t *testing.T) {
	responses := map[string]string{
		"DescribeAutoScalingGroups": `<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups>
      <member>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <AutoScalingGroupARN>arn:asg</AutoScalingGroupARN>
        <MinSize>1</MinSize>
        <MaxSize>3</MaxSize>
        <DesiredCapacity>2</DesiredCapacity>
      </member>
    </AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`,
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
		"DescribeScalingActivities": `<DescribeScalingActivitiesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeScalingActivitiesResult>
    <Activities>
      <member>
        <ActivityId>act-1</ActivityId>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <StatusCode>Successful</StatusCode>
        <Cause>test</Cause>
      </member>
    </Activities>
  </DescribeScalingActivitiesResult>
</DescribeScalingActivitiesResponse>`,
	}
	client := newASGTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListASGs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"autoScalingGroupNames": []string{"asg-1"},
		"limit":                 1,
	}}); err != nil {
		t.Fatalf("list asgs with names: %v", err)
	}
	if _, err := svc.handleListAutoScalingPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"autoScalingGroupName": "asg-1",
		"limit":                1,
	}}); err != nil {
		t.Fatalf("list scaling policies with group: %v", err)
	}
	if _, err := svc.handleGetAutoScalingPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"autoScalingGroupName": "asg-1",
	}}); err != nil {
		t.Fatalf("get scaling policy by group: %v", err)
	}
	if _, err := svc.handleListScalingActivities(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"autoScalingGroupName": "asg-1",
		"activityIds":          []string{"act-1"},
		"limit":                1,
	}}); err != nil {
		t.Fatalf("list scaling activities with filters: %v", err)
	}
	if _, err := svc.handleGetScalingActivity(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"activityId":           "act-1",
		"autoScalingGroupName": "asg-1",
	}}); err != nil {
		t.Fatalf("get scaling activity with group: %v", err)
	}
}

func TestEC2ELBListFilters(t *testing.T) {
	responses := map[string]string{
		"DescribeLoadBalancers": `<DescribeLoadBalancersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeLoadBalancersResult>
    <LoadBalancers>
      <member>
        <LoadBalancerArn>arn:lb</LoadBalancerArn>
        <LoadBalancerName>lb-1</LoadBalancerName>
        <DNSName>lb.aws</DNSName>
        <Scheme>internet-facing</Scheme>
        <VpcId>vpc-1</VpcId>
        <Type>application</Type>
        <State><Code>active</Code></State>
      </member>
    </LoadBalancers>
  </DescribeLoadBalancersResult>
</DescribeLoadBalancersResponse>`,
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
		"DescribeRules": `<DescribeRulesResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeRulesResult>
    <Rules>
      <member>
        <RuleArn>arn:rule</RuleArn>
        <Priority>1</Priority>
        <IsDefault>false</IsDefault>
      </member>
    </Rules>
  </DescribeRulesResult>
</DescribeRulesResponse>`,
		"DescribeTargetHealth": `<DescribeTargetHealthResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeTargetHealthResult>
    <TargetHealthDescriptions>
      <member>
        <Target><Id>i-1</Id><Port>80</Port></Target>
        <TargetHealth><State>healthy</State></TargetHealth>
      </member>
    </TargetHealthDescriptions>
  </DescribeTargetHealthResult>
</DescribeTargetHealthResponse>`,
	}
	client := newELBTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListLoadBalancers(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"loadBalancerArns": []string{"arn:lb"},
		"names":            []string{"lb-1"},
		"limit":            1,
	}}); err != nil {
		t.Fatalf("list load balancers with filters: %v", err)
	}
	if _, err := svc.handleListTargetGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"loadBalancerArn": "arn:lb",
		"limit":           1,
	}}); err != nil {
		t.Fatalf("list target groups with lb: %v", err)
	}
	if _, err := svc.handleListListeners(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"loadBalancerArn": "arn:lb",
		"limit":           1,
	}}); err != nil {
		t.Fatalf("list listeners with lb: %v", err)
	}
	if _, err := svc.handleListListenerRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"listenerArn": "arn:listener",
	}}); err != nil {
		t.Fatalf("list listener rules: %v", err)
	}
	if _, err := svc.handleGetTargetHealth(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"targetGroupArn": "arn:tg",
		"targetIds":      []string{"i-1"},
	}}); err != nil {
		t.Fatalf("get target health with ids: %v", err)
	}
}
