package awsiam

import (
	"context"
	"encoding/json"
	"testing"
)

func TestIAMPrunePolicyVersionsNoOp(t *testing.T) {
	responses := map[string]string{
		"ListPolicyVersions": listPolicyVersionsResponse(4),
	}
	client := newIAMTestClient(t, responses)
	if err := prunePolicyVersions(context.Background(), client, "arn:aws:iam::123:policy/demo"); err != nil {
		t.Fatalf("prune policy versions no-op: %v", err)
	}
}

func TestIAMDeleteNonDefaultPolicyVersions(t *testing.T) {
	responses := map[string]string{
		"ListPolicyVersions":  listPolicyVersionsResponse(3),
		"DeletePolicyVersion": `<DeletePolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyVersionResponse>`,
	}
	client := newIAMTestClient(t, responses)
	if err := deleteNonDefaultPolicyVersions(context.Background(), client, "arn:aws:iam::123:policy/demo"); err != nil {
		t.Fatalf("delete non-default versions: %v", err)
	}
}

func TestIAMTypeHelpers(t *testing.T) {
	if got := toInt(json.Number("5"), 1); got != 5 {
		t.Fatalf("unexpected toInt: %d", got)
	}
	if got := toBool(nil, true); got != true {
		t.Fatalf("unexpected toBool fallback: %v", got)
	}
}
