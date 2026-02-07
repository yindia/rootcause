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
	}
	client := newEC2TestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
		resolverClient: func(context.Context, string) (*route53resolver.Client, string, error) {
			return nil, "", nil
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
	if _, err := svc.handleListRouteTables(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list route tables: %v", err)
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
