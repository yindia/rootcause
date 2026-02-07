package awsec2

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

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2HandlersWithStubbedClient(t *testing.T) {
	responses := map[string]string{
		"DescribeInstances": `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
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
</DescribeInstancesResponse>`,
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
		"DescribeSecurityGroups": `<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <securityGroupInfo>
    <item>
      <groupId>sg-1</groupId>
      <ipPermissions>
        <item>
          <ipProtocol>tcp</ipProtocol>
          <fromPort>80</fromPort>
          <toPort>80</toPort>
          <ipRanges><item><cidrIp>0.0.0.0/0</cidrIp></item></ipRanges>
        </item>
      </ipPermissions>
    </item>
  </securityGroupInfo>
</DescribeSecurityGroupsResponse>`,
		"DescribeSpotInstanceRequests": `<DescribeSpotInstanceRequestsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <spotInstanceRequestSet>
    <item>
      <spotInstanceRequestId>sir-1</spotInstanceRequestId>
      <state>active</state>
      <type>one-time</type>
    </item>
  </spotInstanceRequestSet>
</DescribeSpotInstanceRequestsResponse>`,
		"DescribeCapacityReservations": `<DescribeCapacityReservationsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <capacityReservationSet>
    <item>
      <capacityReservationId>cr-1</capacityReservationId>
      <instanceType>t3.micro</instanceType>
      <availabilityZone>us-east-1a</availabilityZone>
      <totalInstanceCount>2</totalInstanceCount>
      <availableInstanceCount>1</availableInstanceCount>
      <state>active</state>
    </item>
  </capacityReservationSet>
</DescribeCapacityReservationsResponse>`,
		"DescribeVolumes": `<DescribeVolumesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <volumeSet>
    <item>
      <volumeId>vol-1</volumeId>
      <availabilityZone>us-east-1a</availabilityZone>
      <size>10</size>
      <status>in-use</status>
      <volumeType>gp3</volumeType>
      <attachmentSet>
        <item>
          <instanceId>i-1</instanceId>
          <state>attached</state>
          <device>/dev/xvda</device>
        </item>
      </attachmentSet>
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
		"DescribePlacementGroups": `<DescribePlacementGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <placementGroupSet>
    <item>
      <groupName>pg-1</groupName>
      <state>available</state>
      <strategy>cluster</strategy>
    </item>
  </placementGroupSet>
</DescribePlacementGroupsResponse>`,
		"DescribeInstanceStatus": `<DescribeInstanceStatusResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <instanceStatusSet>
    <item>
      <instanceId>i-1</instanceId>
      <availabilityZone>us-east-1a</availabilityZone>
      <instanceState><name>running</name></instanceState>
      <systemStatus><status>ok</status></systemStatus>
      <instanceStatus><status>ok</status></instanceStatus>
    </item>
  </instanceStatusSet>
</DescribeInstanceStatusResponse>`,
	}
	client := newEC2TestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListInstances(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if _, err := svc.handleGetInstance(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}}); err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if _, err := svc.handleListLaunchTemplates(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list launch templates: %v", err)
	}
	if _, err := svc.handleGetLaunchTemplate(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"launchTemplateId": "lt-1"}}); err != nil {
		t.Fatalf("get launch template: %v", err)
	}
	if _, err := svc.handleGetSecurityGroupRules(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupId": "sg-1"}}); err != nil {
		t.Fatalf("get security group rules: %v", err)
	}
	if _, err := svc.handleListSpotInstanceRequests(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list spot instance requests: %v", err)
	}
	if _, err := svc.handleGetSpotInstanceRequest(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"spotInstanceRequestId": "sir-1"}}); err != nil {
		t.Fatalf("get spot instance request: %v", err)
	}
	if _, err := svc.handleListCapacityReservations(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list capacity reservations: %v", err)
	}
	if _, err := svc.handleGetCapacityReservation(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"capacityReservationId": "cr-1"}}); err != nil {
		t.Fatalf("get capacity reservation: %v", err)
	}
	if _, err := svc.handleGetVolume(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"volumeId": "vol-1"}}); err != nil {
		t.Fatalf("get volume: %v", err)
	}
	if _, err := svc.handleGetSnapshot(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"snapshotId": "snap-1"}}); err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if _, err := svc.handleListVolumeAttachments(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list volume attachments: %v", err)
	}
	if _, err := svc.handleGetPlacementGroup(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"groupName": "pg-1"}}); err != nil {
		t.Fatalf("get placement group: %v", err)
	}
	if _, err := svc.handleListPlacementGroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list placement groups: %v", err)
	}
	if _, err := svc.handleListVolumes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list volumes: %v", err)
	}
	if _, err := svc.handleListSnapshots(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if _, err := svc.handleListInstanceStatus(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list instance status: %v", err)
	}
	if _, err := svc.handleGetInstanceStatus(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}}); err != nil {
		t.Fatalf("get instance status: %v", err)
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
