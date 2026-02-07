package awseks

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSHandlersWithStubbedClient(t *testing.T) {
	responses := map[string]string{
		"/clusters":                     `{"clusters":["demo"]}`,
		"/clusters/demo":                `{"cluster":{"name":"demo","arn":"arn:aws:eks:us-east-1:123:cluster/demo","version":"1.28","status":"ACTIVE"}}`,
		"/clusters/demo/addons":         `{"addons":["vpc-cni"]}`,
		"/clusters/demo/addons/vpc-cni": `{"addon":{"addonName":"vpc-cni","addonVersion":"1.12","status":"ACTIVE","health":{"issues":[]}}}`,
	}
	client := newEKSTestClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListClusters(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 1}}); err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	if _, err := svc.handleGetCluster(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"name": "demo"}}); err != nil {
		t.Fatalf("get cluster: %v", err)
	}
	if _, err := svc.handleListAddons(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo"}}); err != nil {
		t.Fatalf("list addons: %v", err)
	}
	if _, err := svc.handleGetAddon(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "addonName": "vpc-cni"}}); err != nil {
		t.Fatalf("get addon: %v", err)
	}
}

func newEKSTestClient(t *testing.T, responses map[string]string) *eks.Client {
	t.Helper()
	transport := &eksRoundTripper{responses: responses}
	cfg := aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", ""),
		HTTPClient:  &http.Client{Transport: transport},
	}
	cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "https://eks.test", SigningRegion: region, HostnameImmutable: true}, nil
		},
	)
	return eks.NewFromConfig(cfg)
}

type eksRoundTripper struct {
	responses map[string]string
}

func (rt *eksRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, ok := rt.responses[req.URL.Path]
	if !ok {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown path")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Request:    req,
	}, nil
}
