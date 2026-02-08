package awsiam

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestIAMGetInstanceProfileMissing(t *testing.T) {
	responses := map[string]string{
		"GetInstanceProfile": `<GetInstanceProfileResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetInstanceProfileResult></GetInstanceProfileResult>
</GetInstanceProfileResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleIAMGetInstanceProfile(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceProfileName": "missing"}}); err != nil {
		t.Fatalf("get instance profile missing: %v", err)
	}
}

func TestIAMDeletePolicyNoForce(t *testing.T) {
	responses := map[string]string{
		"DeletePolicy": `<DeletePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleIAMDeletePolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyArn": "arn:aws:iam::123:policy/demo",
		"confirm":   true,
	}}); err != nil {
		t.Fatalf("delete policy no force: %v", err)
	}
}
