package awskms

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestKMSHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	called := false
	svc := &Service{
		ctx: ctx,
		kmsClient: func(context.Context, string) (*kms.Client, string, error) {
			called = true
			return nil, "", nil
		},
	}

	tests := []struct {
		name    string
		handler func(context.Context, mcp.ToolRequest) (mcp.ToolResult, error)
		args    map[string]any
		wantErr string
	}{
		{"describeKeyMissing", svc.handleDescribeKey, map[string]any{}, "keyId is required"},
		{"getKeyPolicyMissing", svc.handleGetKeyPolicy, map[string]any{}, "keyId is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			_, err := tt.handler(context.Background(), mcp.ToolRequest{Arguments: tt.args})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error %q, got %v", tt.wantErr, err)
			}
			if called {
				t.Fatalf("client should not be invoked")
			}
		})
	}
}

func TestKMSHandlersWithStubbedClient(t *testing.T) {
	responses := map[string]string{
		"TrentService.ListKeys":     `{"Keys":[{"KeyId":"key-1","KeyArn":"arn:aws:kms:us-east-1:123:key/key-1"}]}`,
		"TrentService.ListAliases":  `{"Aliases":[{"AliasName":"alias/demo","AliasArn":"arn:aws:kms:us-east-1:123:alias/demo","TargetKeyId":"key-1"}]}`,
		"TrentService.DescribeKey":  `{"KeyMetadata":{"KeyId":"key-1","Arn":"arn:aws:kms:us-east-1:123:key/key-1","KeyState":"Enabled","KeyUsage":"ENCRYPT_DECRYPT","Origin":"AWS_KMS","CreationDate":1704067200}}`,
		"TrentService.GetKeyPolicy": `{"Policy":"policy"}`,
	}
	client := newKMSTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		kmsClient: func(context.Context, string) (*kms.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListKeys(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if _, err := svc.handleListAliases(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if _, err := svc.handleDescribeKey(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"keyId": "key-1"}}); err != nil {
		t.Fatalf("describe key: %v", err)
	}
	if _, err := svc.handleGetKeyPolicy(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"keyId": "key-1"}}); err != nil {
		t.Fatalf("get key policy: %v", err)
	}
}

func newKMSTestClient(t *testing.T, responses map[string]string) *kms.Client {
	t.Helper()
	transport := &kmsTargetRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://kms.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return kms.NewFromConfig(cfg)
}

type kmsTargetRoundTripper struct {
	responses map[string]string
}

func (rt *kmsTargetRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	target := req.Header.Get("X-Amz-Target")
	resp, ok := rt.responses[target]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown target")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.TrimSpace(resp))),
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Request:    req,
	}, nil
}
