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
