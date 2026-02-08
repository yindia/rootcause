package awsec2

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2ListPagination(t *testing.T) {
	responses := map[string][]string{
		"DescribeInstances": {
			`<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-1</instanceId>
          <imageId>ami-1</imageId>
          <instanceType>t3.micro</instanceType>
          <placement><availabilityZone>us-east-1a</availabilityZone></placement>
          <privateIpAddress>10.0.0.1</privateIpAddress>
          <subnetId>subnet-1</subnetId>
          <vpcId>vpc-1</vpcId>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
  <nextToken>token-1</nextToken>
</DescribeInstancesResponse>`,
			`<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet></reservationSet>
</DescribeInstancesResponse>`,
		},
		"DescribeLaunchTemplates": {
			`<DescribeLaunchTemplatesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <launchTemplates>
    <item>
      <launchTemplateId>lt-1</launchTemplateId>
      <launchTemplateName>tmpl</launchTemplateName>
    </item>
  </launchTemplates>
  <nextToken>token-2</nextToken>
</DescribeLaunchTemplatesResponse>`,
			`<DescribeLaunchTemplatesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <launchTemplates></launchTemplates>
</DescribeLaunchTemplatesResponse>`,
		},
		"DescribeSpotInstanceRequests": {
			`<DescribeSpotInstanceRequestsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <spotInstanceRequestSet>
    <item>
      <spotInstanceRequestId>sir-1</spotInstanceRequestId>
      <state>active</state>
    </item>
  </spotInstanceRequestSet>
  <nextToken>token-3</nextToken>
</DescribeSpotInstanceRequestsResponse>`,
			`<DescribeSpotInstanceRequestsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <spotInstanceRequestSet></spotInstanceRequestSet>
</DescribeSpotInstanceRequestsResponse>`,
		},
		"DescribeVolumes": {
			`<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet>
    <item>
      <volumeId>vol-1</volumeId>
      <availabilityZone>us-east-1a</availabilityZone>
      <size>10</size>
      <status>in-use</status>
      <volumeType>gp3</volumeType>
    </item>
  </volumeSet>
  <nextToken>token-4</nextToken>
</DescribeVolumesResponse>`,
			`<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet></volumeSet>
</DescribeVolumesResponse>`,
		},
		"DescribeSnapshots": {
			`<DescribeSnapshotsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <snapshotSet>
    <item>
      <snapshotId>snap-1</snapshotId>
      <volumeId>vol-1</volumeId>
      <status>completed</status>
      <volumeSize>10</volumeSize>
    </item>
  </snapshotSet>
  <nextToken>token-5</nextToken>
</DescribeSnapshotsResponse>`,
			`<DescribeSnapshotsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <snapshotSet></snapshotSet>
</DescribeSnapshotsResponse>`,
		},
	}
	client := newEC2SequenceClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListInstances(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list instances pagination: %v", err)
	}
	if _, err := svc.handleListLaunchTemplates(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list launch templates pagination: %v", err)
	}
	if _, err := svc.handleListSpotInstanceRequests(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list spot requests pagination: %v", err)
	}
	if _, err := svc.handleListVolumes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list volumes pagination: %v", err)
	}
	if _, err := svc.handleListSnapshots(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list snapshots pagination: %v", err)
	}
}

func TestELBListPagination(t *testing.T) {
	responses := map[string][]string{
		"DescribeLoadBalancers": {
			`<DescribeLoadBalancersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
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
    <NextMarker>marker-1</NextMarker>
  </DescribeLoadBalancersResult>
</DescribeLoadBalancersResponse>`,
			`<DescribeLoadBalancersResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeLoadBalancersResult>
    <LoadBalancers></LoadBalancers>
  </DescribeLoadBalancersResult>
</DescribeLoadBalancersResponse>`,
		},
	}
	client := newELBSequenceClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleListLoadBalancers(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list load balancers pagination: %v", err)
	}
}

func TestASGListPagination(t *testing.T) {
	responses := map[string][]string{
		"DescribeAutoScalingGroups": {
			`<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups>
      <member>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <AutoScalingGroupARN>arn:asg</AutoScalingGroupARN>
      </member>
    </AutoScalingGroups>
    <NextToken>token-1</NextToken>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`,
			`<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups></AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`,
		},
		"DescribePolicies": {
			`<DescribePoliciesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribePoliciesResult>
    <ScalingPolicies>
      <member>
        <PolicyName>policy-1</PolicyName>
        <PolicyARN>arn:policy</PolicyARN>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <PolicyType>TargetTrackingScaling</PolicyType>
      </member>
    </ScalingPolicies>
    <NextToken>token-2</NextToken>
  </DescribePoliciesResult>
</DescribePoliciesResponse>`,
			`<DescribePoliciesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribePoliciesResult>
    <ScalingPolicies></ScalingPolicies>
  </DescribePoliciesResult>
</DescribePoliciesResponse>`,
		},
		"DescribeScalingActivities": {
			`<DescribeScalingActivitiesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeScalingActivitiesResult>
    <Activities>
      <member>
        <ActivityId>act-1</ActivityId>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <StatusCode>Successful</StatusCode>
        <Cause>test</Cause>
      </member>
    </Activities>
    <NextToken>token-3</NextToken>
  </DescribeScalingActivitiesResult>
</DescribeScalingActivitiesResponse>`,
			`<DescribeScalingActivitiesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeScalingActivitiesResult>
    <Activities></Activities>
  </DescribeScalingActivitiesResult>
</DescribeScalingActivitiesResponse>`,
		},
		"DescribeLaunchConfigurations": {
			`<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations>
      <member>
        <LaunchConfigurationName>lc-1</LaunchConfigurationName>
        <ImageId>ami-1</ImageId>
        <InstanceType>t3.micro</InstanceType>
      </member>
    </LaunchConfigurations>
    <NextToken>token-4</NextToken>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
			`<DescribeLaunchConfigurationsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeLaunchConfigurationsResult>
    <LaunchConfigurations></LaunchConfigurations>
  </DescribeLaunchConfigurationsResult>
</DescribeLaunchConfigurationsResponse>`,
		},
	}
	client := newASGSequenceClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleListASGs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list asgs pagination: %v", err)
	}
	if _, err := svc.handleListAutoScalingPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list autoscaling policies pagination: %v", err)
	}
	if _, err := svc.handleListScalingActivities(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list scaling activities pagination: %v", err)
	}
	if _, err := svc.handleListLaunchConfigurations(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list launch configurations pagination: %v", err)
	}
}

func newEC2SequenceClient(t *testing.T, responses map[string][]string) *ec2.Client {
	t.Helper()
	transport := &sequenceRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://ec2.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return ec2.NewFromConfig(cfg)
}

func newASGSequenceClient(t *testing.T, responses map[string][]string) *autoscaling.Client {
	t.Helper()
	transport := &sequenceRoundTripper{responses: responses}
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

func newELBSequenceClient(t *testing.T, responses map[string][]string) *elasticloadbalancingv2.Client {
	t.Helper()
	transport := &sequenceRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://elb.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return elasticloadbalancingv2.NewFromConfig(cfg)
}

type sequenceRoundTripper struct {
	mu        sync.Mutex
	responses map[string][]string
	index     map[string]int
}

func (rt *sequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	values, _ := url.ParseQuery(string(body))
	action := values.Get("Action")
	if action == "" {
		action = req.URL.Query().Get("Action")
	}
	rt.mu.Lock()
	if rt.index == nil {
		rt.index = map[string]int{}
	}
	idx := rt.index[action]
	respList := rt.responses[action]
	if len(respList) == 0 {
		rt.mu.Unlock()
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown action")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	if idx >= len(respList) {
		idx = len(respList) - 1
	}
	rt.index[action] = idx + 1
	resp := strings.TrimSpace(respList[idx])
	rt.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Request:    req,
	}, nil
}
