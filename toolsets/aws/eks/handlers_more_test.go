package awseks

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSListNodesWithNodegroup(t *testing.T) {
	eksResponses := map[string]string{
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

	if _, err := svc.handleListNodes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"clusterName":  "demo",
		"nodegroupName": "ng-1",
		"limit":        1,
	}}); err != nil {
		t.Fatalf("list nodes with nodegroup: %v", err)
	}
}

func TestEKSListNodesMissingCluster(t *testing.T) {
	svc := &Service{ctx: mcp.ToolsetContext{Redactor: redact.New()}}
	if _, err := svc.handleListNodes(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}}); err == nil {
		t.Fatalf("expected error for missing clusterName")
	}
}
