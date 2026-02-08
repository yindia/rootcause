package awsec2

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2ELBHandlersWithStubbedClient(t *testing.T) {
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
        <DefaultActions>
          <member>
            <Type>forward</Type>
            <TargetGroupArn>arn:tg</TargetGroupArn>
          </member>
        </DefaultActions>
      </member>
    </Listeners>
  </DescribeListenersResult>
</DescribeListenersResponse>`,
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
		"DescribeRules": `<DescribeRulesResponse xmlns="http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/">
  <DescribeRulesResult>
    <Rules>
      <member>
        <RuleArn>arn:rule</RuleArn>
        <Priority>1</Priority>
        <IsDefault>false</IsDefault>
        <Actions>
          <member>
            <Type>forward</Type>
            <TargetGroupArn>arn:tg</TargetGroupArn>
          </member>
        </Actions>
      </member>
    </Rules>
  </DescribeRulesResult>
</DescribeRulesResponse>`,
	}
	client := newELBTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		elbClient: func(context.Context, string) (*elasticloadbalancingv2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListLoadBalancers(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list load balancers: %v", err)
	}
	if _, err := svc.handleGetLoadBalancer(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"loadBalancerArn": "arn:lb"}}); err != nil {
		t.Fatalf("get load balancer: %v", err)
	}
	if _, err := svc.handleListTargetGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list target groups: %v", err)
	}
	if _, err := svc.handleGetTargetGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"targetGroupArn": "arn:tg"}}); err != nil {
		t.Fatalf("get target group: %v", err)
	}
	if _, err := svc.handleListListeners(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list listeners: %v", err)
	}
	if _, err := svc.handleGetListener(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"listenerArn": "arn:listener"}}); err != nil {
		t.Fatalf("get listener: %v", err)
	}
	if _, err := svc.handleGetTargetHealth(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"targetGroupArn": "arn:tg"}}); err != nil {
		t.Fatalf("get target health: %v", err)
	}
	if _, err := svc.handleListListenerRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"listenerArn": "arn:listener"}}); err != nil {
		t.Fatalf("list listener rules: %v", err)
	}
	if _, err := svc.handleGetListenerRule(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"ruleArn": "arn:rule"}}); err != nil {
		t.Fatalf("get listener rule: %v", err)
	}
}

func newELBTestClient(t *testing.T, responses map[string]string) *elasticloadbalancingv2.Client {
	t.Helper()
	transport := &queryRoundTripper{responses: responses}
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
