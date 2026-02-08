package karpenter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAWSHelperUtilities(t *testing.T) {
	if got := awsNameFromARN("arn:aws:iam::123:role/MyRole", ":role/"); got != "MyRole" {
		t.Fatalf("unexpected awsNameFromARN: %s", got)
	}
	if got := awsNameFromARN("role-name", ":role/"); got != "role-name" {
		t.Fatalf("expected passthrough awsNameFromARN")
	}

	match := resourceMatch{Group: "karpenter.k8s.aws", Kind: "EC2NodeClass"}
	if !isAWSNodeClass(match, nil) {
		t.Fatalf("expected AWS nodeclass match")
	}
	if isAWSNodeClass(resourceMatch{Group: "example.com", Kind: "NodeClass"}, nil) {
		t.Fatalf("did not expect nodeclass match")
	}
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("karpenter.k8s.aws/v1")
	if !isAWSNodeClass(resourceMatch{Group: "", Kind: ""}, obj) {
		t.Fatalf("expected nodeclass match from apiVersion")
	}
}

func TestFilterByNameAndConditions(t *testing.T) {
	obj1 := unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "one"}}}
	obj2 := unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"name": "two"}}}
	if got := filterByName([]unstructured.Unstructured{obj1, obj2}, ""); len(got) != 2 {
		t.Fatalf("expected filterByName to return all")
	}
	if got := filterByName([]unstructured.Unstructured{obj1, obj2}, "two"); len(got) != 1 {
		t.Fatalf("expected filterByName to return match")
	}
	condObj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": "True", "reason": "ok"},
				"bad",
			},
		},
	}}
	conditions := extractConditions(condObj)
	if len(conditions) != 1 {
		t.Fatalf("expected extractConditions to parse valid condition")
	}
}
