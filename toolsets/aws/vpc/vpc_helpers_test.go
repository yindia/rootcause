package awsvpc

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestTypeHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toString(10); got != "10" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toStringSlice([]any{"a", " ", "b"}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toStringSlice("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected toStringSlice string: %#v", got)
	}
	if got := toInt(json.Number("7"), 1); got != 7 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toInt("bad", 2); got != 2 {
		t.Fatalf("expected fallback toInt")
	}
	if got := regionOrDefault(""); got != "us-east-1" {
		t.Fatalf("expected default region")
	}
}

func TestTagFiltersFromArgs(t *testing.T) {
	filters := tagFiltersFromArgs(map[string]any{
		"env":  []any{"prod", "staging"},
		"tier": "backend",
	})
	if len(filters) != 2 {
		t.Fatalf("expected filters, got %#v", filters)
	}
}

func TestSummarizers(t *testing.T) {
	vpc := ec2types.Vpc{
		VpcId:     aws.String("vpc-1"),
		CidrBlock: aws.String("10.0.0.0/16"),
		IsDefault: aws.Bool(true),
		Tags: []ec2types.Tag{
			{Key: aws.String("env"), Value: aws.String("dev")},
		},
	}
	vpcSummary := summarizeVPC(vpc)
	if vpcSummary["id"] != "vpc-1" {
		t.Fatalf("unexpected vpc summary: %#v", vpcSummary)
	}
	subnet := ec2types.Subnet{
		SubnetId:         aws.String("subnet-1"),
		VpcId:            aws.String("vpc-1"),
		CidrBlock:        aws.String("10.0.1.0/24"),
		AvailabilityZone: aws.String("us-east-1a"),
	}
	subnetSummary := summarizeSubnet(subnet)
	if subnetSummary["id"] != "subnet-1" {
		t.Fatalf("unexpected subnet summary: %#v", subnetSummary)
	}
}
