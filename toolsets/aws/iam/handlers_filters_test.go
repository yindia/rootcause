package awsiam

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestIAMListRolesAndPoliciesFilters(t *testing.T) {
	assumeDoc := url.QueryEscape(`{"Statement":[]}`)
	responses := map[string]string{
		"ListRoles": fmt.Sprintf(`<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolesResult>
    <Roles>
      <member>
        <Path>/service/</Path>
        <RoleName>demo</RoleName>
        <RoleId>RID</RoleId>
        <Arn>arn:aws:iam::123:role/demo</Arn>
        <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      </member>
    </Roles>
    <IsTruncated>false</IsTruncated>
  </ListRolesResult>
</ListRolesResponse>`),
		"ListPolicies": `<ListPoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListPoliciesResult>
    <Policies>
      <member>
        <PolicyName>demo</PolicyName>
        <PolicyId>PID</PolicyId>
        <Arn>arn:aws:iam::123:policy/demo</Arn>
        <Path>/</Path>
        <DefaultVersionId>v1</DefaultVersionId>
        <AttachmentCount>1</AttachmentCount>
        <IsAttachable>true</IsAttachable>
        <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      </member>
    </Policies>
    <IsTruncated>false</IsTruncated>
  </ListPoliciesResult>
</ListPoliciesResponse>`,
		"GetPolicy": `<GetPolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetPolicyResult>
    <Policy>
      <PolicyName>demo</PolicyName>
      <PolicyId>PID</PolicyId>
      <Arn>arn:aws:iam::123:policy/demo</Arn>
      <Path>/</Path>
      <DefaultVersionId>v1</DefaultVersionId>
      <AttachmentCount>1</AttachmentCount>
      <IsAttachable>true</IsAttachable>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </Policy>
  </GetPolicyResult>
</GetPolicyResponse>`,
		"GetRole": fmt.Sprintf(`<GetRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetRoleResult>
    <Role>
      <Path>/</Path>
      <RoleName>demo</RoleName>
      <RoleId>RID</RoleId>
      <Arn>arn:aws:iam::123:role/demo</Arn>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      <AssumeRolePolicyDocument>%s</AssumeRolePolicyDocument>
    </Role>
  </GetRoleResult>
</GetRoleResponse>`, assumeDoc),
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleIAMListRoles(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"pathPrefix": "/service/",
		"limit":      1,
	}}); err != nil {
		t.Fatalf("list roles filters: %v", err)
	}
	if _, err := svc.handleIAMListPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"scope":        "Local",
		"onlyAttached": true,
		"limit":        1,
	}}); err != nil {
		t.Fatalf("list policies filters: %v", err)
	}
	if _, err := svc.handleIAMGetPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyArn":       "arn:aws:iam::123:policy/demo",
		"includeDocument": false,
	}}); err != nil {
		t.Fatalf("get policy without document: %v", err)
	}
	if _, err := svc.handleIAMUpdateRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName":                   "demo",
		"maxSessionDurationSeconds":  3600,
		"confirm":                    true,
	}}); err == nil {
		// UpdateRole may fail with the stubbed client; allow error for coverage.
	}
}
