package awsiam

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
}

func newIAMTestClient(t *testing.T, responses map[string]string) *iam.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		values, _ := url.ParseQuery(string(body))
		action := values.Get("Action")
		if action == "" {
			action = r.URL.Query().Get("Action")
		}
		resp, ok := responses[action]
		if !ok {
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(strings.TrimSpace(resp)))
	}))
	t.Cleanup(server.Close)

	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  server.Client(),
	}
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{URL: server.URL, SigningRegion: region, HostnameImmutable: true}, nil
	})
	cfg.EndpointResolverWithOptions = resolver
	return iam.NewFromConfig(cfg)
}
