package awsvpc

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestVPCGetNotFoundBranches(t *testing.T) {
	responses := map[string]string{
		"DescribeVpcs": `<DescribeVpcsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcSet></vpcSet>
</DescribeVpcsResponse>`,
		"DescribeSubnets": `<DescribeSubnetsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <subnetSet></subnetSet>
</DescribeSubnetsResponse>`,
		"DescribeRouteTables": `<DescribeRouteTablesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <routeTableSet></routeTableSet>
</DescribeRouteTablesResponse>`,
		"DescribeNatGateways": `<DescribeNatGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <natGatewaySet></natGatewaySet>
</DescribeNatGatewaysResponse>`,
		"DescribeSecurityGroups": `<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo></securityGroupInfo>
</DescribeSecurityGroupsResponse>`,
		"DescribeNetworkAcls": `<DescribeNetworkAclsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkAclSet></networkAclSet>
</DescribeNetworkAclsResponse>`,
		"DescribeInternetGateways": `<DescribeInternetGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <internetGatewaySet></internetGatewaySet>
</DescribeInternetGatewaysResponse>`,
		"DescribeVpcEndpoints": `<DescribeVpcEndpointsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcEndpointSet></vpcEndpointSet>
</DescribeVpcEndpointsResponse>`,
		"DescribeNetworkInterfaces": `<DescribeNetworkInterfacesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkInterfaceSet></networkInterfaceSet>
</DescribeNetworkInterfacesResponse>`,
	}
	client := newEC2TestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleGetVPC(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"vpcId": "vpc-1"}}); err == nil {
		t.Fatalf("expected vpc not found")
	}
	if _, err := svc.handleGetSubnet(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"subnetId": "subnet-1"}}); err == nil {
		t.Fatalf("expected subnet not found")
	}
	if _, err := svc.handleGetRouteTable(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"routeTableId": "rtb-1"}}); err == nil {
		t.Fatalf("expected route table not found")
	}
	if _, err := svc.handleGetNatGateway(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"natGatewayId": "nat-1"}}); err == nil {
		t.Fatalf("expected nat gateway not found")
	}
	if _, err := svc.handleGetSecurityGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupId": "sg-1"}}); err == nil {
		t.Fatalf("expected security group not found")
	}
	if _, err := svc.handleGetNetworkAcl(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"networkAclId": "acl-1"}}); err == nil {
		t.Fatalf("expected network acl not found")
	}
	if _, err := svc.handleGetInternetGateway(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"internetGatewayId": "igw-1"}}); err == nil {
		t.Fatalf("expected internet gateway not found")
	}
	if _, err := svc.handleGetEndpoint(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"endpointId": "vpce-1"}}); err == nil {
		t.Fatalf("expected endpoint not found")
	}
	if _, err := svc.handleGetNetworkInterface(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"networkInterfaceId": "eni-1"}}); err == nil {
		t.Fatalf("expected network interface not found")
	}
}

func TestVPCResolverClientMissing(t *testing.T) {
	svc := &Service{ctx: mcp.ToolsetContext{Redactor: redact.New()}}
	if _, err := svc.handleGetResolverEndpoint(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"resolverEndpointId": "rslvr-endpoint-1"}}); err == nil {
		t.Fatalf("expected resolver client error")
	}
	if _, err := svc.handleGetResolverRule(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"resolverRuleId": "rslvr-rule-1"}}); err == nil {
		t.Fatalf("expected resolver client error")
	}
}
