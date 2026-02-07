package karpenter

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"rootcause/internal/mcp"
	"rootcause/internal/render"
)

func TestAddAWSNodeClassEvidenceNoRegistry(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "karpenter.k8s.aws/v1beta1",
			"kind":       "EC2NodeClass",
			"metadata":   map[string]any{"name": "default"},
			"spec": map[string]any{
				"subnetSelectorTerms": []any{
					map[string]any{"ids": []any{"subnet-123"}},
				},
			},
		},
	}
	toolset := New()
	toolset.ctx = mcp.ToolsetContext{}
	analysis := render.NewAnalysis()
	toolset.addAWSNodeClassEvidence(context.Background(), mcp.ToolRequest{}, &analysis, resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}, obj)
	if len(analysis.Evidence) == 0 {
		t.Fatalf("expected evidence from addAWSNodeClassEvidence")
	}
}

func TestAWSNameFromARN(t *testing.T) {
	if got := awsNameFromARN("arn:aws:ec2:us-east-1:123456789012:subnet/subnet-123", ":subnet/"); got != "subnet-123" {
		t.Fatalf("unexpected aws name: %s", got)
	}
	if got := awsNameFromARN("invalid", ":subnet/"); got != "invalid" {
		t.Fatalf("expected passthrough aws name")
	}
}
