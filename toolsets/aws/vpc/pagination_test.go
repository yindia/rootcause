package awsvpc

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
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestVPCListPagination(t *testing.T) {
	responses := map[string][]string{
		"DescribeVpcs": {
			`<DescribeVpcsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcSet>
    <item>
      <vpcId>vpc-1</vpcId>
      <cidrBlock>10.0.0.0/16</cidrBlock>
    </item>
  </vpcSet>
  <nextToken>token-1</nextToken>
</DescribeVpcsResponse>`,
			`<DescribeVpcsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcSet></vpcSet>
</DescribeVpcsResponse>`,
		},
		"DescribeSubnets": {
			`<DescribeSubnetsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <subnetSet>
    <item>
      <subnetId>subnet-1</subnetId>
      <vpcId>vpc-1</vpcId>
      <cidrBlock>10.0.1.0/24</cidrBlock>
      <availabilityZone>us-east-1a</availabilityZone>
    </item>
  </subnetSet>
  <nextToken>token-2</nextToken>
</DescribeSubnetsResponse>`,
			`<DescribeSubnetsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <subnetSet></subnetSet>
</DescribeSubnetsResponse>`,
		},
		"DescribeRouteTables": {
			`<DescribeRouteTablesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
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
    </item>
  </routeTableSet>
  <nextToken>token-rtb</nextToken>
</DescribeRouteTablesResponse>`,
			`<DescribeRouteTablesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <routeTableSet></routeTableSet>
</DescribeRouteTablesResponse>`,
		},
		"DescribeNatGateways": {
			`<DescribeNatGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <natGatewaySet>
    <item>
      <natGatewayId>nat-1</natGatewayId>
      <vpcId>vpc-1</vpcId>
      <subnetId>subnet-1</subnetId>
      <state>available</state>
    </item>
  </natGatewaySet>
  <nextToken>token-nat</nextToken>
</DescribeNatGatewaysResponse>`,
			`<DescribeNatGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <natGatewaySet></natGatewaySet>
</DescribeNatGatewaysResponse>`,
		},
		"DescribeSecurityGroups": {
			`<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo>
    <item>
      <groupId>sg-1</groupId>
      <groupName>web</groupName>
      <description>demo</description>
      <vpcId>vpc-1</vpcId>
    </item>
  </securityGroupInfo>
  <nextToken>token-sg</nextToken>
</DescribeSecurityGroupsResponse>`,
			`<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo></securityGroupInfo>
</DescribeSecurityGroupsResponse>`,
		},
		"DescribeNetworkInterfaces": {
			`<DescribeNetworkInterfacesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkInterfaceSet>
    <item>
      <networkInterfaceId>eni-1</networkInterfaceId>
      <status>in-use</status>
      <description>demo</description>
      <vpcId>vpc-1</vpcId>
      <subnetId>subnet-1</subnetId>
      <privateIpAddress>10.0.0.10</privateIpAddress>
    </item>
  </networkInterfaceSet>
  <nextToken>token-eni</nextToken>
</DescribeNetworkInterfacesResponse>`,
			`<DescribeNetworkInterfacesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkInterfaceSet></networkInterfaceSet>
</DescribeNetworkInterfacesResponse>`,
		},
		"DescribeNetworkAcls": {
			`<DescribeNetworkAclsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkAclSet>
    <item>
      <networkAclId>acl-1</networkAclId>
      <vpcId>vpc-1</vpcId>
      <isDefault>true</isDefault>
    </item>
  </networkAclSet>
  <nextToken>token-acl</nextToken>
</DescribeNetworkAclsResponse>`,
			`<DescribeNetworkAclsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <networkAclSet></networkAclSet>
</DescribeNetworkAclsResponse>`,
		},
		"DescribeInternetGateways": {
			`<DescribeInternetGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <internetGatewaySet>
    <item>
      <internetGatewayId>igw-1</internetGatewayId>
    </item>
  </internetGatewaySet>
  <nextToken>token-igw</nextToken>
</DescribeInternetGatewaysResponse>`,
			`<DescribeInternetGatewaysResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <internetGatewaySet></internetGatewaySet>
</DescribeInternetGatewaysResponse>`,
		},
		"DescribeVpcEndpoints": {
			`<DescribeVpcEndpointsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcEndpointSet>
    <item>
      <vpcEndpointId>vpce-1</vpcEndpointId>
      <vpcId>vpc-1</vpcId>
      <serviceName>com.amazonaws.vpce</serviceName>
      <vpcEndpointType>Interface</vpcEndpointType>
      <state>Available</state>
    </item>
  </vpcEndpointSet>
  <nextToken>token-ep</nextToken>
</DescribeVpcEndpointsResponse>`,
			`<DescribeVpcEndpointsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <vpcEndpointSet></vpcEndpointSet>
</DescribeVpcEndpointsResponse>`,
		},
	}
	ec2Client := newVPCSequenceClient(t, responses)
	resolverResponses := map[string][]string{
		"Route53Resolver.ListResolverEndpoints": {
			`{"ResolverEndpoints":[{"Id":"rslvr-endpoint-1","Name":"resolver","Direction":"INBOUND","Status":"OPERATIONAL","HostVPCId":"vpc-1","SecurityGroupIds":["sg-1"],"IpAddressCount":1,"CreationTime":"2024-01-01T00:00:00Z"}],"NextToken":"token-3"}`,
			`{"ResolverEndpoints":[]}`,
		},
	}
	resolverClient := newResolverSequenceClient(t, resolverResponses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2Client, "us-east-1", nil
		},
		resolverClient: func(context.Context, string) (*route53resolver.Client, string, error) {
			return resolverClient, "us-east-1", nil
		},
	}

	if _, err := svc.handleListVPCs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list vpcs pagination: %v", err)
	}
	if _, err := svc.handleListSubnets(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list subnets pagination: %v", err)
	}
	if _, err := svc.handleListRouteTables(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list route tables pagination: %v", err)
	}
	if _, err := svc.handleListNatGateways(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list nat gateways pagination: %v", err)
	}
	if _, err := svc.handleListSecurityGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list security groups pagination: %v", err)
	}
	if _, err := svc.handleListNetworkInterfaces(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list network interfaces pagination: %v", err)
	}
	if _, err := svc.handleListNetworkAcls(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list network acls pagination: %v", err)
	}
	if _, err := svc.handleListInternetGateways(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list internet gateways pagination: %v", err)
	}
	if _, err := svc.handleListEndpoints(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list endpoints pagination: %v", err)
	}
	if _, err := svc.handleListResolverEndpoints(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list resolver endpoints pagination: %v", err)
	}
}

func newVPCSequenceClient(t *testing.T, responses map[string][]string) *ec2.Client {
	t.Helper()
	transport := &vpcSequenceRoundTripper{responses: responses}
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

func newResolverSequenceClient(t *testing.T, responses map[string][]string) *route53resolver.Client {
	t.Helper()
	transport := &vpcSequenceRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://resolver.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return route53resolver.NewFromConfig(cfg)
}

type vpcSequenceRoundTripper struct {
	mu        sync.Mutex
	responses map[string][]string
	index     map[string]int
}

func (rt *vpcSequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	action := req.Header.Get("X-Amz-Target")
	if action == "" {
		body, _ := io.ReadAll(req.Body)
		_ = req.Body.Close()
		values, _ := url.ParseQuery(string(body))
		if parsed := values.Get("Action"); parsed != "" {
			action = parsed
		}
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
	contentType := "text/xml"
	if strings.HasPrefix(resp, "{") {
		contentType = "application/x-amz-json-1.1"
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     http.Header{"Content-Type": []string{contentType}},
		Request:    req,
	}, nil
}
