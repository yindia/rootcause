package awsec2

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2AutoScalingHandlersWithStubbedClient(t *testing.T) {
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
        <VPCZoneIdentifier>subnet-1,subnet-2</VPCZoneIdentifier>
        <HealthCheckType>EC2</HealthCheckType>
        <HealthCheckGracePeriod>300</HealthCheckGracePeriod>
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
		"DescribeLaunchConfigurations": `<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations>
      <member>
        <LaunchConfigurationName>lc-1</LaunchConfigurationName>
        <ImageId>ami-1</ImageId>
        <InstanceType>t3.micro</InstanceType>
        <IamInstanceProfile>profile</IamInstanceProfile>
      </member>
    </LaunchConfigurations>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
	}
	client := newASGTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListASGs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list asgs: %v", err)
	}
	if _, err := svc.handleGetASG(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"autoScalingGroupName": "asg-1"}}); err != nil {
		t.Fatalf("get asg: %v", err)
	}
	if _, err := svc.handleListAutoScalingPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list scaling policies: %v", err)
	}
	if _, err := svc.handleGetAutoScalingPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"policyName": "policy-1"}}); err != nil {
		t.Fatalf("get scaling policy: %v", err)
	}
	if _, err := svc.handleListScalingActivities(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list scaling activities: %v", err)
	}
	if _, err := svc.handleGetScalingActivity(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"activityId": "act-1"}}); err != nil {
		t.Fatalf("get scaling activity: %v", err)
	}
	if _, err := svc.handleListLaunchConfigurations(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list launch configurations: %v", err)
	}
	if _, err := svc.handleGetLaunchConfiguration(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"launchConfigurationName": "lc-1"}}); err != nil {
		t.Fatalf("get launch configuration: %v", err)
	}
}

func newASGTestClient(t *testing.T, responses map[string]string) *autoscaling.Client {
	t.Helper()
	transport := &queryRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://autoscaling.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return autoscaling.NewFromConfig(cfg)
}
