package awseks

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func TestSummarizersExtended(t *testing.T) {
	now := time.Now()
	nodegroup := ekstypes.Nodegroup{
		NodegroupName:  aws.String("ng-1"),
		NodegroupArn:   aws.String("arn:ng"),
		Status:         ekstypes.NodegroupStatusActive,
		Version:        aws.String("1.28"),
		CapacityType:   ekstypes.CapacityTypesOnDemand,
		AmiType:        ekstypes.AMITypesAl2X8664,
		ReleaseVersion: aws.String("1.28.1"),
		NodeRole:       aws.String("arn:role"),
		Subnets:        []string{"subnet-1"},
		InstanceTypes:  []string{"t3.medium"},
		Labels:         map[string]string{"team": "ops"},
		ScalingConfig:  &ekstypes.NodegroupScalingConfig{DesiredSize: aws.Int32(2)},
	}
	nodegroupSummary := summarizeNodegroup(nodegroup)
	if nodegroupSummary["name"] != "ng-1" {
		t.Fatalf("unexpected nodegroup summary: %#v", nodegroupSummary)
	}

	fargate := ekstypes.FargateProfile{
		FargateProfileName:  aws.String("fg"),
		FargateProfileArn:   aws.String("arn:fg"),
		Status:              ekstypes.FargateProfileStatusActive,
		Selectors:           []ekstypes.FargateProfileSelector{{Namespace: aws.String("default")}},
		Subnets:             []string{"subnet-1"},
		PodExecutionRoleArn: aws.String("arn:role"),
		CreatedAt:           &now,
		Tags:                map[string]string{"env": "dev"},
	}
	fargateSummary := summarizeFargateProfile(fargate)
	if fargateSummary["name"] != "fg" {
		t.Fatalf("unexpected fargate summary: %#v", fargateSummary)
	}

	idp := ekstypes.IdentityProviderConfigResponse{
		Oidc: &ekstypes.OidcIdentityProviderConfig{
			ClientId: aws.String("client"),
		},
	}
	idpSummary := summarizeIdentityProviderConfig(idp)
	if _, ok := idpSummary["oidc"]; !ok {
		t.Fatalf("expected oidc summary: %#v", idpSummary)
	}

	update := ekstypes.Update{
		Id:        aws.String("upd-1"),
		Type:      ekstypes.UpdateTypeVersionUpdate,
		Status:    ekstypes.UpdateStatusSuccessful,
		CreatedAt: &now,
	}
	updateSummary := summarizeUpdate(update)
	if updateSummary["id"] != "upd-1" {
		t.Fatalf("unexpected update summary: %#v", updateSummary)
	}

	instance := ec2types.Instance{
		InstanceId:       aws.String("i-1"),
		State:            &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		VpcId:            aws.String("vpc-1"),
		SubnetId:         aws.String("subnet-1"),
		Placement:        &ec2types.Placement{AvailabilityZone: aws.String("us-east-1a")},
		PrivateIpAddress: aws.String("10.0.0.1"),
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-1")},
		},
		Tags: []ec2types.Tag{{Key: aws.String("env"), Value: aws.String("dev")}},
	}
	instSummary := summarizeInstance(instance, "ng-1")
	if instSummary["nodegroup"] != "ng-1" {
		t.Fatalf("unexpected instance summary: %#v", instSummary)
	}
}
