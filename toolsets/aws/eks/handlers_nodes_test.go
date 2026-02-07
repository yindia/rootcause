package awseks

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSListNodesWithStubbedClients(t *testing.T) {
	eksResponses := map[string]string{
		"/clusters/demo/node-groups":      `{"nodegroups":["ng-1"]}`,
		"/clusters/demo/node-groups/ng-1": `{"nodegroup":{"nodegroupName":"ng-1","resources":{"autoScalingGroups":[{"name":"asg-1"}]}}}`,
	}
	eksClient := newEKSTestClient(t, eksResponses)

	asgResponses := map[string]string{
		"DescribeAutoScalingGroups": `<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups>
      <member>
        <AutoScalingGroupName>asg-1</AutoScalingGroupName>
        <Instances>
          <member><InstanceId>i-1</InstanceId></member>
        </Instances>
      </member>
    </AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
</DescribeAutoScalingGroupsResponse>`,
	}
	asgClient := newASGTestClient(t, asgResponses)

	ec2Responses := map[string]string{
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
	}
	ec2Client := newEC2TestClient(t, ec2Responses)

	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			return eksClient, "us-east-1", nil
		},
		asgClient: func(context.Context, string) (*autoscaling.Client, string, error) {
			return asgClient, "us-east-1", nil
		},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2Client, "us-east-1", nil
		},
	}

	result, err := svc.handleListNodes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo"}})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["count"] == nil {
		t.Fatalf("expected result count, got %#v", result.Data)
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
