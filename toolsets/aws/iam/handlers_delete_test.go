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

func TestIAMDeleteRoleForce(t *testing.T) {
	responses := map[string]string{
		"ListAttachedRolePolicies": `<ListAttachedRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListAttachedRolePoliciesResult>
    <AttachedPolicies>
      <member><PolicyName>p1</PolicyName><PolicyArn>arn:aws:iam::123:policy/p1</PolicyArn></member>
    </AttachedPolicies>
    <IsTruncated>false</IsTruncated>
  </ListAttachedRolePoliciesResult>
</ListAttachedRolePoliciesResponse>`,
		"ListRolePolicies": `<ListRolePoliciesResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListRolePoliciesResult>
    <PolicyNames><member>inline1</member></PolicyNames>
    <IsTruncated>false</IsTruncated>
  </ListRolePoliciesResult>
</ListRolePoliciesResponse>`,
		"ListInstanceProfilesForRole": `<ListInstanceProfilesForRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListInstanceProfilesForRoleResult>
    <InstanceProfiles>
      <member>
        <InstanceProfileName>profile</InstanceProfileName>
        <Arn>arn:aws:iam::123:instance-profile/profile</Arn>
      </member>
    </InstanceProfiles>
    <IsTruncated>false</IsTruncated>
  </ListInstanceProfilesForRoleResult>
</ListInstanceProfilesForRoleResponse>`,
		"DetachRolePolicy":              `<DetachRolePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DetachRolePolicyResponse>`,
		"DeleteRolePolicy":              `<DeleteRolePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeleteRolePolicyResponse>`,
		"RemoveRoleFromInstanceProfile": `<RemoveRoleFromInstanceProfileResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></RemoveRoleFromInstanceProfileResponse>`,
		"DeleteRole":                    `<DeleteRoleResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeleteRoleResponse>`,
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
		"force":    true,
	}}); err != nil {
		t.Fatalf("delete role: %v", err)
	}
}

func TestIAMDeletePolicyForce(t *testing.T) {
	responses := map[string]string{
		"ListPolicyVersions": `<ListPolicyVersionsResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListPolicyVersionsResult>
    <Versions>
      <member><VersionId>v1</VersionId><IsDefaultVersion>true</IsDefaultVersion><CreateDate>2024-01-02T00:00:00Z</CreateDate></member>
      <member><VersionId>v2</VersionId><IsDefaultVersion>false</IsDefaultVersion><CreateDate>2024-01-01T00:00:00Z</CreateDate></member>
    </Versions>
  </ListPolicyVersionsResult>
</ListPolicyVersionsResponse>`,
		"DeletePolicyVersion": `<DeletePolicyVersionResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyVersionResponse>`,
		"DeletePolicy":        `<DeletePolicyResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/"></DeletePolicyResponse>`,
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
		"force":     true,
	}}); err != nil {
		t.Fatalf("delete policy: %v", err)
	}
}

func TestIAMPrunePolicyVersions(t *testing.T) {
	client := newIAMSequenceClient(t)
	if err := prunePolicyVersions(context.Background(), client, "arn:aws:iam::123:policy/demo"); err != nil {
		t.Fatalf("prune policy versions: %v", err)
	}
}

func newIAMSequenceClient(t *testing.T) *iam.Client {
	t.Helper()
	transport := &iamSequenceRoundTripper{}
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

type iamSequenceRoundTripper struct {
	mu        sync.Mutex
	listCalls int
}

func (rt *iamSequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
	default:
		return xmlResponse(req, `<ErrorResponse><Error><Code>UnknownAction</Code></Error></ErrorResponse>`), nil
	}
}

func listPolicyVersionsResponse(count int) string {
	versions := []string{
		`<member><VersionId>v1</VersionId><IsDefaultVersion>true</IsDefaultVersion><CreateDate>2024-01-05T00:00:00Z</CreateDate></member>`,
		`<member><VersionId>v2</VersionId><IsDefaultVersion>false</IsDefaultVersion><CreateDate>2024-01-04T00:00:00Z</CreateDate></member>`,
		`<member><VersionId>v3</VersionId><IsDefaultVersion>false</IsDefaultVersion><CreateDate>2024-01-03T00:00:00Z</CreateDate></member>`,
		`<member><VersionId>v4</VersionId><IsDefaultVersion>false</IsDefaultVersion><CreateDate>2024-01-02T00:00:00Z</CreateDate></member>`,
		`<member><VersionId>v5</VersionId><IsDefaultVersion>false</IsDefaultVersion><CreateDate>2024-01-01T00:00:00Z</CreateDate></member>`,
	}
	if count < len(versions) {
		versions = versions[:count]
	}
	return `<ListPolicyVersionsResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <ListPolicyVersionsResult>
    <Versions>` + strings.Join(versions, "") + `</Versions>
  </ListPolicyVersionsResult>
</ListPolicyVersionsResponse>`
}

func xmlResponse(req *http.Request, payload string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.TrimSpace(payload))),
		Header:     http.Header{"Content-Type": []string{"text/xml"}},
		Request:    req,
	}
}
