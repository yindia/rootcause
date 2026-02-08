package awssts

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestSTSHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	called := false
	svc := &Service{
		ctx: ctx,
		stsClient: func(context.Context, string) (*sts.Client, string, error) {
			called = true
			return nil, "", nil
		},
	}
	_, err := svc.handleAssumeRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"confirm": true}})
	if err == nil || !strings.Contains(err.Error(), "roleArn is required") {
		t.Fatalf("expected roleArn error, got %v", err)
	}
	if called {
		t.Fatalf("client should not be invoked")
	}
}

func TestSTSHandlersWithStubbedClient(t *testing.T) {
	responses := map[string]string{
		"GetCallerIdentity": `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123:user/demo</Arn>
    <Account>123</Account>
    <UserId>ABC</UserId>
  </GetCallerIdentityResult>
</GetCallerIdentityResponse>`,
		"AssumeRole": `<AssumeRoleResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <AssumeRoleResult>
    <Credentials>
      <AccessKeyId>AKID</AccessKeyId>
      <SecretAccessKey>SECRET</SecretAccessKey>
      <SessionToken>token</SessionToken>
      <Expiration>2024-01-01T00:00:00Z</Expiration>
    </Credentials>
    <AssumedRoleUser>
      <Arn>arn:aws:sts::123:assumed-role/demo/session</Arn>
      <AssumedRoleId>AROA:session</AssumedRoleId>
    </AssumedRoleUser>
  </AssumeRoleResult>
</AssumeRoleResponse>`,
	}
	client := newSTSTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		stsClient: func(context.Context, string) (*sts.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleGetCallerIdentity(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}}); err != nil {
		t.Fatalf("get caller identity: %v", err)
	}
	if _, err := svc.handleAssumeRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleArn":     "arn:aws:iam::123:role/demo",
		"sessionName": "session",
		"confirm":     true,
	}}); err != nil {
		t.Fatalf("assume role: %v", err)
	}
	if _, err := svc.handleAssumeRole(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"roleArn":     "arn:aws:iam::123:role/demo",
		"sessionName": "session",
	}}); err == nil {
		t.Fatalf("expected confirm error")
	}
}

func newSTSTestClient(t *testing.T, responses map[string]string) *sts.Client {
	t.Helper()
	transport := &stsQueryRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://sts.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return sts.NewFromConfig(cfg)
}

type stsQueryRoundTripper struct {
	responses map[string]string
}

func (rt *stsQueryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
