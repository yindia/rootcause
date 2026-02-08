package awsiam

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestIAMHandlersWithStubbedClient(t *testing.T) {
	assumeDoc := url.QueryEscape(`{"Statement":[]}`)
	policyDoc := url.QueryEscape(`{"Version":"2012-10-17","Statement":[]}`)
	responses := map[string]string{
		"ListRoles": fmt.Sprintf(
			`<ListRolesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolesResult>
    <Roles>
      <member>
        <Path>/</Path>
        <RoleName>demo</RoleName>
        <RoleId>RID</RoleId>
        <Arn>arn:aws:iam::123:role/demo</Arn>
        <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      </member>
    </Roles>
    <IsTruncated>false</IsTruncated>
  </ListRolesResult>
  <ResponseMetadata><RequestId>req</RequestId></ResponseMetadata>
</ListRolesResponse>`,
		),
		"GetRole": fmt.Sprintf(
			`<GetRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
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
  <ResponseMetadata><RequestId>req</RequestId></ResponseMetadata>
</GetRoleResponse>`, assumeDoc,
		),
		"UpdateRole": `<UpdateRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <UpdateRoleResult>
    <Role>
      <Path>/</Path>
      <RoleName>demo</RoleName>
      <RoleId>RID</RoleId>
      <Arn>arn:aws:iam::123:role/demo</Arn>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </Role>
  </UpdateRoleResult>
</UpdateRoleResponse>`,
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
  <ResponseMetadata><RequestId>req</RequestId></ResponseMetadata>
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
		"GetPolicyVersion": fmt.Sprintf(`<GetPolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetPolicyVersionResult>
    <PolicyVersion>
      <Document>%s</Document>
      <VersionId>v1</VersionId>
      <IsDefaultVersion>true</IsDefaultVersion>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </PolicyVersion>
  </GetPolicyVersionResult>
</GetPolicyVersionResponse>`, policyDoc),
		"CreatePolicyVersion": `<CreatePolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <CreatePolicyVersionResult>
    <PolicyVersion>
      <VersionId>v2</VersionId>
      <IsDefaultVersion>true</IsDefaultVersion>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </PolicyVersion>
  </CreatePolicyVersionResult>
</CreatePolicyVersionResponse>`,
		"DeletePolicy": `<DeletePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyResponse>`,
		"GetInstanceProfile": `<GetInstanceProfileResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetInstanceProfileResult>
    <InstanceProfile>
      <Path>/</Path>
      <InstanceProfileName>profile</InstanceProfileName>
      <InstanceProfileId>IPID</InstanceProfileId>
      <Arn>arn:aws:iam::123:instance-profile/profile</Arn>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      <Roles>
        <member>
          <Path>/</Path>
          <RoleName>demo</RoleName>
          <RoleId>RID</RoleId>
          <Arn>arn:aws:iam::123:role/demo</Arn>
          <CreateDate>2024-01-01T00:00:00Z</CreateDate>
        </member>
      </Roles>
    </InstanceProfile>
  </GetInstanceProfileResult>
</GetInstanceProfileResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	result, err := svc.handleIAMListRoles(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}})
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", result.Data)
	}
	roles, ok := data["roles"].([]map[string]any)
	if !ok || len(roles) != 1 {
		t.Fatalf("expected roles, got %#v", data["roles"])
	}

	roleResult, err := svc.handleIAMGetRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName":        "demo",
		"includePolicies": false,
	}})
	if err != nil {
		t.Fatalf("get role: %v", err)
	}
	roleData, ok := roleResult.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", roleResult.Data)
	}
	if roleData["role"] == nil {
		t.Fatalf("expected role data")
	}
	if roleData["assumeRolePolicy"] == nil {
		t.Fatalf("expected assume role policy")
	}

	if _, err := svc.handleIAMListPolicies(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list policies: %v", err)
	}
	if _, err := svc.handleIAMGetPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"policyArn": "arn:aws:iam::123:policy/demo"}}); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if _, err := svc.handleIAMUpdatePolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyArn": "arn:aws:iam::123:policy/demo",
		"document":  `{"Version":"2012-10-17","Statement":[]}`,
		"confirm":   true,
	}}); err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if _, err := svc.handleIAMDeletePolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyArn": "arn:aws:iam::123:policy/demo",
		"confirm":   true,
	}}); err != nil {
		t.Fatalf("delete policy: %v", err)
	}
	if _, err := svc.handleIAMGetInstanceProfile(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceProfileName": "profile"}}); err != nil {
		t.Fatalf("get instance profile: %v", err)
	}
	if _, err := svc.handleIAMUpdateRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName":    "demo",
		"description": "updated",
		"confirm":     true,
	}}); err != nil {
		t.Fatalf("update role: %v", err)
	}
}

func newIAMTestClient(t *testing.T, responses map[string]string) *iam.Client {
	t.Helper()
	transport := &iamRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://iam.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return iam.NewFromConfig(cfg)
}

type iamRoundTripper struct {
	responses map[string]string
}

func (rt *iamRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	values, _ := url.ParseQuery(string(body))
	action := values.Get("Action")
	if action == "" {
		action = req.URL.Query().Get("Action")
	}
	resp, ok := rt.responses[action]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown action")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.TrimSpace(resp))),
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Request:    req,
	}, nil
}
