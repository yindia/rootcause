package awseks

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/eks"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSListPagination(t *testing.T) {
	responses := map[string][]string{
		"/clusters": {
			`{"clusters":["demo"],"nextToken":"token-1"}`,
			`{"clusters":[]}`,
		},
		"/clusters/demo/node-groups": {
			`{"nodegroups":["ng-1"],"nextToken":"token-2"}`,
			`{"nodegroups":[]}`,
		},
		"/clusters/demo/addons": {
			`{"addons":["vpc-cni"],"nextToken":"token-3"}`,
			`{"addons":[]}`,
		},
		"/clusters/demo/fargate-profiles": {
			`{"fargateProfileNames":["fp-1"],"nextToken":"token-4"}`,
			`{"fargateProfileNames":[]}`,
		},
		"/clusters/demo/updates": {
			`{"updateIds":["upd-1"],"nextToken":"token-5"}`,
			`{"updateIds":[]}`,
		},
		"/clusters/demo/identity-provider-configs": {
			`{"identityProviderConfigs":[{"type":"oidc","name":"idp"}],"nextToken":"token-6"}`,
			`{"identityProviderConfigs":[]}`,
		},
	}
	client := newEKSSequenceClient(t, responses)
	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			return client, "us-east-1", nil
		},
	}

	if _, err := svc.handleListClusters(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"limit": 10}}); err != nil {
		t.Fatalf("list clusters pagination: %v", err)
	}
	if _, err := svc.handleListNodegroups(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "limit": 10}}); err != nil {
		t.Fatalf("list nodegroups pagination: %v", err)
	}
	if _, err := svc.handleListAddons(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "limit": 10}}); err != nil {
		t.Fatalf("list addons pagination: %v", err)
	}
	if _, err := svc.handleListFargateProfiles(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "limit": 10}}); err != nil {
		t.Fatalf("list fargate profiles pagination: %v", err)
	}
	if _, err := svc.handleListUpdates(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "limit": 10}}); err != nil {
		t.Fatalf("list updates pagination: %v", err)
	}
	if _, err := svc.handleListIdentityProviderConfigs(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"clusterName": "demo", "limit": 10}}); err != nil {
		t.Fatalf("list identity provider configs pagination: %v", err)
	}
}

func newEKSSequenceClient(t *testing.T, responses map[string][]string) *eks.Client {
	t.Helper()
	transport := &eksSequenceRoundTripper{responses: responses}
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

type eksSequenceRoundTripper struct {
	mu        sync.Mutex
	responses map[string][]string
	index     map[string]int
}

func (rt *eksSequenceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	if rt.index == nil {
		rt.index = map[string]int{}
	}
	path := req.URL.Path
	idx := rt.index[path]
	respList := rt.responses[path]
	if len(respList) == 0 {
		rt.mu.Unlock()
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader("unknown path")),
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Request:    req,
		}, nil
	}
	if idx >= len(respList) {
		idx = len(respList) - 1
	}
	rt.index[path] = idx + 1
	resp := strings.TrimSpace(respList[idx])
	rt.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(resp)),
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Request:    req,
	}, nil
}
