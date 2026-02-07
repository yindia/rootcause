package awsvpc

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestVPCHandlersWithStubbedClient(t *testing.T) {
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
		"DescribeNatGateways": `<DescribeNatGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <natGatewaySet>
    <item>
      <natGatewayId>nat-1</natGatewayId>
      <vpcId>vpc-1</vpcId>
      <subnetId>subnet-1</subnetId>
      <state>available</state>
      <createTime>2024-01-01T00:00:00Z</createTime>
    </item>
  </natGatewaySet>
</DescribeNatGatewaysResponse>`,
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
		"DescribeRouteTables": `<DescribeRouteTablesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <routeTableSet>
    <item>
      <routeTableId>rtb-1</routeTableId>
      <vpcId>vpc-1</vpcId>
      <routeSet>
        <item>
          <destinationCidrBlock>0.0.0.0/0</destinationCidrBlock>
          <gatewayId>igw-1</gatewayId>
          <state>active</state>
          <origin>CreateRoute</origin>
        </item>
      </routeSet>
      <associationSet>
        <item>
          <routeTableAssociationId>rtbassoc-1</routeTableAssociationId>
          <subnetId>subnet-1</subnetId>
          <main>true</main>
        </item>
      </associationSet>
    </item>
  </routeTableSet>
</DescribeRouteTablesResponse>`,
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
		"DescribeNetworkAcls": `<DescribeNetworkAclsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkAclSet>
    <item>
      <networkAclId>acl-1</networkAclId>
      <vpcId>vpc-1</vpcId>
      <isDefault>true</isDefault>
      <entrySet>
        <item>
          <ruleNumber>100</ruleNumber>
          <protocol>6</protocol>
          <ruleAction>allow</ruleAction>
          <egress>false</egress>
          <cidrBlock>0.0.0.0/0</cidrBlock>
        </item>
      </entrySet>
    </item>
  </networkAclSet>
</DescribeNetworkAclsResponse>`,
		"DescribeInternetGateways": `<DescribeInternetGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <internetGatewaySet>
    <item>
      <internetGatewayId>igw-1</internetGatewayId>
      <attachmentSet>
        <item>
          <vpcId>vpc-1</vpcId>
          <state>available</state>
        </item>
      </attachmentSet>
    </item>
  </internetGatewaySet>
</DescribeInternetGatewaysResponse>`,
		"DescribeVpcEndpoints": `<DescribeVpcEndpointsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcEndpointSet>
    <item>
      <vpcEndpointId>vpce-1</vpcEndpointId>
      <vpcId>vpc-1</vpcId>
      <serviceName>com.amazonaws.vpce</serviceName>
      <vpcEndpointType>Interface</vpcEndpointType>
      <state>Available</state>
      <subnetIdSet>
        <item>subnet-1</item>
      </subnetIdSet>
    </item>
  </vpcEndpointSet>
</DescribeVpcEndpointsResponse>`,
		"DescribeNetworkInterfaces": `<DescribeNetworkInterfacesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkInterfaceSet>
    <item>
      <networkInterfaceId>eni-1</networkInterfaceId>
      <status>in-use</status>
      <description>demo</description>
      <vpcId>vpc-1</vpcId>
      <subnetId>subnet-1</subnetId>
      <privateIpAddress>10.0.0.10</privateIpAddress>
      <groupSet>
        <item><groupId>sg-1</groupId></item>
      </groupSet>
    </item>
  </networkInterfaceSet>
</DescribeNetworkInterfacesResponse>`,
	}
	client := newEC2TestClient(t, responses)
	resolverResponses := map[string]string{
		"Route53Resolver.ListResolverEndpoints": `{"ResolverEndpoints":[{"Id":"rslvr-endpoint-1","Name":"resolver","Direction":"INBOUND","Status":"OPERATIONAL","HostVPCId":"vpc-1","SecurityGroupIds":["sg-1"],"IpAddressCount":1,"CreationTime":"2024-01-01T00:00:00Z"}]}`,
		"Route53Resolver.GetResolverEndpoint":   `{"ResolverEndpoint":{"Id":"rslvr-endpoint-1","Name":"resolver","Direction":"INBOUND","Status":"OPERATIONAL","HostVPCId":"vpc-1","SecurityGroupIds":["sg-1"],"IpAddressCount":1,"CreationTime":"2024-01-01T00:00:00Z"}}`,
		"Route53Resolver.ListResolverRules":     `{"ResolverRules":[{"Id":"rslvr-rule-1","Name":"rule","DomainName":"corp.local","RuleType":"FORWARD","Status":"COMPLETE","ResolverEndpointId":"rslvr-endpoint-1","TargetIps":[{"Ip":"10.0.0.2","Port":53}]}]}`,
		"Route53Resolver.GetResolverRule":       `{"ResolverRule":{"Id":"rslvr-rule-1","Name":"rule","DomainName":"corp.local","RuleType":"FORWARD","Status":"COMPLETE","ResolverEndpointId":"rslvr-endpoint-1","TargetIps":[{"Ip":"10.0.0.2","Port":53}]}}`,
	}
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

	if _, err := svc.handleListVPCs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list vpcs: %v", err)
	}
	if _, err := svc.handleGetVPC(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"vpcId": "vpc-1"}}); err != nil {
		t.Fatalf("get vpc: %v", err)
	}
	if _, err := svc.handleListSubnets(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list subnets: %v", err)
	}
	if _, err := svc.handleGetSubnet(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"subnetId": "subnet-1"}}); err != nil {
		t.Fatalf("get subnet: %v", err)
	}
	if _, err := svc.handleListNatGateways(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list nat gateways: %v", err)
	}
	if _, err := svc.handleGetNatGateway(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"natGatewayId": "nat-1"}}); err != nil {
		t.Fatalf("get nat gateway: %v", err)
	}
	if _, err := svc.handleListRouteTables(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list route tables: %v", err)
	}
	if _, err := svc.handleListSecurityGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list security groups: %v", err)
	}
	if _, err := svc.handleGetSecurityGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupId": "sg-1"}}); err != nil {
		t.Fatalf("get security group: %v", err)
	}
	if _, err := svc.handleListNetworkAcls(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list network acls: %v", err)
	}
	if _, err := svc.handleGetNetworkAcl(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"networkAclId": "acl-1"}}); err != nil {
		t.Fatalf("get network acl: %v", err)
	}
	if _, err := svc.handleListInternetGateways(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list internet gateways: %v", err)
	}
	if _, err := svc.handleGetInternetGateway(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"internetGatewayId": "igw-1"}}); err != nil {
		t.Fatalf("get internet gateway: %v", err)
	}
	if _, err := svc.handleListEndpoints(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list endpoints: %v", err)
	}
	if _, err := svc.handleGetEndpoint(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"endpointId": "vpce-1"}}); err != nil {
		t.Fatalf("get endpoint: %v", err)
	}
	if _, err := svc.handleListNetworkInterfaces(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list network interfaces: %v", err)
	}
	if _, err := svc.handleGetNetworkInterface(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"networkInterfaceId": "eni-1"}}); err != nil {
		t.Fatalf("get network interface: %v", err)
	}
	if _, err := svc.handleListResolverEndpoints(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list resolver endpoints: %v", err)
	}
	if _, err := svc.handleGetResolverEndpoint(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"resolverEndpointId": "rslvr-endpoint-1"}}); err != nil {
		t.Fatalf("get resolver endpoint: %v", err)
	}
	if _, err := svc.handleListResolverRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list resolver rules: %v", err)
	}
	if _, err := svc.handleGetResolverRule(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"resolverRuleId": "rslvr-rule-1"}}); err != nil {
		t.Fatalf("get resolver rule: %v", err)
	}
}

func newEC2TestClient(t *testing.T, responses map[string]string) *ec2.Client {
	t.Helper()
	transport := &queryRoundTripper{responses: responses}
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

func newResolverTestClient(t *testing.T, responses map[string]string) *route53resolver.Client {
	t.Helper()
	transport := &resolverRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://route53resolver.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return route53resolver.NewFromConfig(cfg)
}

type queryRoundTripper struct {
	responses map[string]string
}

func (rt *queryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	values, _ := url.ParseQuery(string(body))
	action := values.Get("Action")
	if action == "" {
		action = req.URL.Query().Get("Action")
	}
	resp, ok := rt.responses[action]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown action")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.TrimSpace(resp))),
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Request:    req,
	}, nil
}

type resolverRoundTripper struct {
	responses map[string]string
}

func (rt *resolverRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	target := req.Header.Get("X-Amz-Target")
	resp, ok := rt.responses[target]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown action")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Request:    req,
	}, nil
}
