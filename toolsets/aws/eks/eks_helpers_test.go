package awseks

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func TestTypeHelpers(t *testing.T) {
	if got := toString(nil); got != "" {
		t.Fatalf("expected empty string")
	}
	if got := toString(8); got != "8" {
		t.Fatalf("unexpected toString: %q", got)
	}
	if got := toStringSlice([]any{"a", " ", "b"}); len(got) != 2 {
		t.Fatalf("unexpected toStringSlice: %#v", got)
	}
	if got := toStringSlice("x"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("unexpected toStringSlice string: %#v", got)
	}
	if got := toInt(json.Number("6"), 1); got != 6 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toInt("bad", 2); got != 2 {
		t.Fatalf("expected fallback toInt")
	}
	if got := regionOrDefault(""); got != "us-east-1" {
		t.Fatalf("expected default region")
	}
}

func TestTagMap(t *testing.T) {
	tags := []ec2types.Tag{
		{Key: aws.String("team"), Value: aws.String("ops")},
		{Key: aws.String(""), Value: aws.String("ignored")},
	}
	mapped := tagMap(tags)
	if mapped["team"] != "ops" || len(mapped) != 1 {
		t.Fatalf("unexpected tag map: %#v", mapped)
	}
}

func TestSummarizers(t *testing.T) {
	now := time.Now()
	cluster := ekstypes.Cluster{
		Name:    aws.String("demo"),
		Arn:     aws.String("arn:aws:eks:us-east-1:123:cluster/demo"),
		Version: aws.String("1.28"),
		Status:  ekstypes.ClusterStatusActive,
		CreatedAt: &now,
	}
	clusterSummary := summarizeCluster(cluster)
	if clusterSummary["name"] != "demo" {
		t.Fatalf("unexpected cluster summary: %#v", clusterSummary)
	}
	addon := ekstypes.Addon{
		AddonName: aws.String("vpc-cni"),
		AddonVersion: aws.String("1.12"),
		Status:  ekstypes.AddonStatusActive,
		Health:  &ekstypes.AddonHealth{},
	}
	addonSummary := summarizeAddon(addon)
	if addonSummary["name"] != "vpc-cni" {
		t.Fatalf("unexpected addon summary: %#v", addonSummary)
	}
}
