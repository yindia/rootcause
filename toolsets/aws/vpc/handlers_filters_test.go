package awsvpc

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestVPCListFilters(t *testing.T) {
	responses := map[string]string{
		"DescribeVpcs": `<DescribeVpcsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcSet>
    <item>
      <vpcId>vpc-1</vpcId>
      <cidrBlock>10.0.0.0/16</cidrBlock>
      <isDefault>true</isDefault>
    </item>
  </vpcSet>
</DescribeVpcsResponse>`,
		"DescribeSubnets": `<DescribeSubnetsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <subnetSet>
    <item>
      <subnetId>subnet-1</subnetId>
      <vpcId>vpc-1</vpcId>
      <cidrBlock>10.0.1.0/24</cidrBlock>
      <availabilityZone>us-east-1a</availabilityZone>
    </item>
  </subnetSet>
</DescribeSubnetsResponse>`,
		"DescribeSecurityGroups": `<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo>
    <item>
      <groupId>sg-1</groupId>
      <groupName>web</groupName>
      <description>demo</description>
      <vpcId>vpc-1</vpcId>
    </item>
  </securityGroupInfo>
</DescribeSecurityGroupsResponse>`,
	}
	resolverResponses := map[string]string{
		"Route53Resolver.ListResolverEndpoints": `{"ResolverEndpoints":[{"Id":"rslvr-endpoint-1","Name":"resolver","Direction":"INBOUND","Status":"OPERATIONAL","HostVPCId":"vpc-1","SecurityGroupIds":["sg-1"],"IpAddressCount":1,"CreationTime":"2024-01-01T00:00:00Z"}]}`,
		"Route53Resolver.ListResolverRules":     `{"ResolverRules":[{"Id":"rslvr-rule-1","Name":"rule","DomainName":"corp.local","RuleType":"FORWARD","Status":"COMPLETE","ResolverEndpointId":"rslvr-endpoint-1","TargetIps":[{"Ip":"10.0.0.2","Port":53}]}]}`,
	}
	client := newEC2TestClient(t, responses)
	resolverClient := newResolverTestClient(t, resolverResponses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
		resolverClient: func(context.Context, string) (*route53resolver.Client, string, error) {
			return resolverClient, "us-east-1", nil
		},
	}

	if _, err := svc.handleListVPCs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"vpcIds": []string{"vpc-1"},
		"limit":  1,
	}}); err != nil {
		t.Fatalf("list vpcs with ids: %v", err)
	}
	if _, err := svc.handleListSubnets(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"vpcId":      "vpc-1",
		"subnetIds":  []string{"subnet-1"},
		"tagFilters": map[string]any{"env": []string{"dev"}},
		"limit":      1,
	}}); err != nil {
		t.Fatalf("list subnets with filters: %v", err)
	}
	if _, err := svc.handleListSecurityGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"vpcId":      "vpc-1",
		"groupIds":   []string{"sg-1"},
		"tagFilters": map[string]any{"app": "web"},
		"limit":      1,
	}}); err != nil {
		t.Fatalf("list security groups with filters: %v", err)
	}
	if _, err := svc.handleListResolverEndpoints(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"vpcId": "vpc-1",
		"limit": 1,
	}}); err != nil {
		t.Fatalf("list resolver endpoints with vpc: %v", err)
	}
	if _, err := svc.handleListResolverRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"resolverEndpointId": "rslvr-endpoint-1",
		"ruleType":           "FORWARD",
		"limit":              1,
	}}); err != nil {
		t.Fatalf("list resolver rules with filters: %v", err)
	}
}
