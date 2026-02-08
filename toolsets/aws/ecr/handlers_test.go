package awsecr

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestECRHandlerValidation(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}
	called := false
	svc := &Service{
		ctx: ctx,
		ecrClient: func(context.Context, string) (*ecr.Client, string, error) {
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
		{"describeRepoMissing", svc.handleDescribeRepository, map[string]any{}, "repositoryName is required"},
		{"listImagesMissing", svc.handleListImages, map[string]any{}, "repositoryName is required"},
		{"describeImagesMissing", svc.handleDescribeImages, map[string]any{}, "repositoryName is required"},
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

func TestECRHandlersWithStubbedClient(t *testing.T) {
	responses := map[string]string{
		"AmazonEC2ContainerRegistry_V20150921.DescribeRepositories":  `{"repositories":[{"repositoryName":"demo","repositoryArn":"arn:aws:ecr:us-east-1:123:repository/demo","registryId":"123","repositoryUri":"123.dkr.ecr.us-east-1.amazonaws.com/demo","createdAt":1704067200}]}`,
		"AmazonEC2ContainerRegistry_V20150921.ListImages":            `{"imageIds":[{"imageDigest":"sha256:abc","imageTag":"v1"}]}`,
		"AmazonEC2ContainerRegistry_V20150921.DescribeImages":        `{"imageDetails":[{"imageDigest":"sha256:abc","imageTags":["v1"],"imagePushedAt":1704067200,"imageSizeInBytes":42}]}`,
		"AmazonEC2ContainerRegistry_V20150921.DescribeRegistry":      `{"registryId":"123","replicationConfiguration":{"rules":[]}}`,
		"AmazonEC2ContainerRegistry_V20150921.GetAuthorizationToken": `{"authorizationData":[{"authorizationToken":"token","proxyEndpoint":"https://123.dkr.ecr.us-east-1.amazonaws.com","expiresAt":1704067200}]}`,
	}
	client := newECRTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		ecrClient: func(context.Context, string) (*ecr.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListRepositories(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list repositories: %v", err)
	}
	if _, err := svc.handleDescribeRepository(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"repositoryName": "demo"}}); err != nil {
		t.Fatalf("describe repository: %v", err)
	}
	if _, err := svc.handleListImages(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"repositoryName": "demo", "limit": 1}}); err != nil {
		t.Fatalf("list images: %v", err)
	}
	if _, err := svc.handleDescribeImages(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"repositoryName": "demo", "limit": 1}}); err != nil {
		t.Fatalf("describe images: %v", err)
	}
	if _, err := svc.handleDescribeRegistry(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}}); err != nil {
		t.Fatalf("describe registry: %v", err)
	}
	if _, err := svc.handleGetAuthorizationToken(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"confirm": true}}); err != nil {
		t.Fatalf("get auth token: %v", err)
	}
	if _, err := svc.handleGetAuthorizationToken(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}}); err == nil {
		t.Fatalf("expected confirm error")
	}
}

func newECRTestClient(t *testing.T, responses map[string]string) *ecr.Client {
	t.Helper()
	transport := &ecrTargetRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://ecr.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return ecr.NewFromConfig(cfg)
}

type ecrTargetRoundTripper struct {
	responses map[string]string
}

func (rt *ecrTargetRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
