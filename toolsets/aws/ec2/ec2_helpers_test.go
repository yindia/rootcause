package awsec2

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	autotypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestTypeHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toString(5); got != "5" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toStringSlice([]any{"a", " ", "b"}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toStringSlice("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected toStringSlice string: %#v", got)
	}
	if got := toBool(nil, true); got != true {
		t.Fatalf("expected fallback toBool")
	}
	if got := toBool(false, true); got != false {
		t.Fatalf("expected false toBool")
	}
	if got := toInt(json.Number("9"), 1); got != 9 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toInt("bad", 2); got != 2 {
		t.Fatalf("expected fallback toInt")
	}
	if got := regionOrDefault(""); got != "us-east-1" {
		t.Fatalf("expected default region")
	}
}

func TestTagMapAutoScaling(t *testing.T) {
	tags := []autotypes.TagDescription{
		{Key: aws.String("env"), Value: aws.String("prod")},
		{Key: aws.String(""), Value: aws.String("ignored")},
	}
	mapped := tagMapAutoScaling(tags)
	if mapped["env"] != "prod" || len(mapped) != 1 {
		t.Fatalf("unexpected tag map: %#v", mapped)
	}
}

func TestSummarizers(t *testing.T) {
	instance := ec2types.Instance{
		InstanceId: aws.String("i-123"),
		State:      &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
		VpcId:      aws.String("vpc-1"),
		SubnetId:   aws.String("subnet-1"),
		Placement:  &ec2types.Placement{AvailabilityZone: aws.String("us-east-1a")},
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: aws.String("sg-1")},
		},
	}
	instSummary := summarizeInstance(instance)
	if instSummary["id"] != "i-123" {
		t.Fatalf("unexpected instance summary: %#v", instSummary)
	}
	asg := autotypes.AutoScalingGroup{
		AutoScalingGroupName: aws.String("asg"),
		Instances: []autotypes.Instance{
			{InstanceId: aws.String("i-123")},
		},
	}
	asgSummary := summarizeASG(asg)
	if asgSummary["name"] != "asg" {
		t.Fatalf("unexpected asg summary: %#v", asgSummary)
	}
}
