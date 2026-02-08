package awsiam

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestIAMGetRoleIncludePolicies(t *testing.T) {
	assumeDoc := url.QueryEscape(`{"Statement":[]}`)
	responses := map[string]string{
		"GetRole": `<GetRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetRoleResult>
    <Role>
      <Path>/</Path>
      <RoleName>demo</RoleName>
      <RoleId>RID</RoleId>
      <Arn>arn:aws:iam::123:role/demo</Arn>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
      <AssumeRolePolicyDocument>` + assumeDoc + `</AssumeRolePolicyDocument>
    </Role>
  </GetRoleResult>
</GetRoleResponse>`,
		"ListAttachedRolePolicies": `<ListAttachedRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListAttachedRolePoliciesResult>
    <AttachedPolicies>
      <member>
        <PolicyName>demo</PolicyName>
        <PolicyArn>arn:aws:iam::123:policy/demo</PolicyArn>
      </member>
    </AttachedPolicies>
  </ListAttachedRolePoliciesResult>
</ListAttachedRolePoliciesResponse>`,
		"ListRolePolicies": `<ListRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolePoliciesResult>
    <PolicyNames>
      <member>inline</member>
    </PolicyNames>
  </ListRolePoliciesResult>
</ListRolePoliciesResponse>`,
		"ListInstanceProfilesForRole": `<ListInstanceProfilesForRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListInstanceProfilesForRoleResult>
    <InstanceProfiles>
      <member>
        <InstanceProfileName>profile</InstanceProfileName>
      </member>
    </InstanceProfiles>
  </ListInstanceProfilesForRoleResult>
</ListInstanceProfilesForRoleResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	result, err := svc.handleIAMGetRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName":        "demo",
		"includePolicies": true,
	}})
	if err != nil {
		t.Fatalf("get role: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", result.Data)
	}
	if data["attachedPolicies"] == nil || data["inlinePolicies"] == nil || data["instanceProfiles"] == nil {
		t.Fatalf("expected policy details, got %#v", data)
	}
}

func TestIAMDeleteRoleRequiresForce(t *testing.T) {
	responses := map[string]string{
		"ListAttachedRolePolicies": `<ListAttachedRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListAttachedRolePoliciesResult>
    <AttachedPolicies>
      <member>
        <PolicyName>demo</PolicyName>
        <PolicyArn>arn:aws:iam::123:policy/demo</PolicyArn>
      </member>
    </AttachedPolicies>
  </ListAttachedRolePoliciesResult>
</ListAttachedRolePoliciesResponse>`,
		"ListRolePolicies": `<ListRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolePoliciesResult></ListRolePoliciesResult>
</ListRolePoliciesResponse>`,
		"ListInstanceProfilesForRole": `<ListInstanceProfilesForRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListInstanceProfilesForRoleResult></ListInstanceProfilesForRoleResult>
</ListInstanceProfilesForRoleResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleIAMDeleteRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName": "demo",
		"confirm":  true,
	}}); err == nil {
		t.Fatalf("expected error without force when attachments exist")
	}
}

func TestIAMUpdateRoleAssumeDoc(t *testing.T) {
	responses := map[string]string{
		"UpdateAssumeRolePolicy": `<UpdateAssumeRolePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></UpdateAssumeRolePolicyResponse>`,
	}
	client := newIAMTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleIAMUpdateRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleName":                 "demo",
		"assumeRolePolicyDocument": `{"Statement":[]}`,
		"confirm":                  true,
	}}); err != nil {
		t.Fatalf("update role assume doc: %v", err)
	}
}

func TestIAMUpdatePolicyWithPrune(t *testing.T) {
	client := newIAMPruneTestClient(t)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return client, "us-east-1", nil
		},
	}
	if _, err := svc.handleIAMUpdatePolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"policyArn": "arn:aws:iam::123:policy/demo",
		"document":  `{"Version":"2012-10-17","Statement":[]}`,
		"confirm":   true,
		"prune":     true,
	}}); err != nil {
		t.Fatalf("update policy with prune: %v", err)
	}
}

func newIAMPruneTestClient(t *testing.T) *iam.Client {
	t.Helper()
	transport := &iamPruneRoundTripper{}
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

type iamPruneRoundTripper struct {
	mu        sync.Mutex
	listCalls int
}

func (rt *iamPruneRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(req.Body)
	_ = req.Body.Close()
	values, _ := url.ParseQuery(string(body))
	action := values.Get("Action")
	if action == "" {
		action = req.URL.Query().Get("Action")
	}
	switch action {
	case "ListPolicyVersions":
		rt.mu.Lock()
		rt.listCalls++
		call := rt.listCalls
		rt.mu.Unlock()
		payload := listPolicyVersionsResponse(5)
		if call > 1 {
			payload = listPolicyVersionsResponse(4)
		}
		return xmlResponse(req, payload), nil
	case "DeletePolicyVersion":
		return xmlResponse(req, `<DeletePolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyVersionResponse>`), nil
	case "CreatePolicyVersion":
		return xmlResponse(req, `<CreatePolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <CreatePolicyVersionResult>
    <PolicyVersion>
      <VersionId>v5</VersionId>
      <IsDefaultVersion>true</IsDefaultVersion>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </PolicyVersion>
  </CreatePolicyVersionResult>
</CreatePolicyVersionResponse>`), nil
	default:
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown action")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
}
