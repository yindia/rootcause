package awseks

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEKSDebugMissingClusterName(t *testing.T) {
	svc := &Service{ctx: mcp.ToolsetContext{Redactor: redact.New()}}
	if _, err := svc.handleDebug(context.Background(), mcp.ToolRequest{Arguments: map[string]any{}}); err == nil {
		t.Fatalf("expected missing clusterName error")
	}
}

func TestEKSDebugWithAWSDependencies(t *testing.T) {
	eksClient := newEKSTestClient(t, map[string]string{
		"/clusters/demo": `{"cluster":{"name":"demo","arn":"arn:aws:eks:us-east-1:123:cluster/demo","version":"1.28","status":"ACTIVE","encryptionConfig":[{"resources":["secrets"],"provider":{"keyArn":"arn:aws:kms:us-east-1:123:key/key-1"}}]}}`,
	})
	stsClient := newSTSTestClient(t, map[string]string{
		"GetCallerIdentity": `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123:user/demo</Arn>
    <Account>123</Account>
    <UserId>ABC</UserId>
  </GetCallerIdentityResult>
</GetCallerIdentityResponse>`,
	})
	kmsClient := newKMSTestClient(t, map[string]string{
		"TrentService.DescribeKey": `{"KeyMetadata":{"KeyId":"key-1","Arn":"arn:aws:kms:us-east-1:123:key/key-1","KeyState":"Enabled","KeyUsage":"ENCRYPT_DECRYPT","Origin":"AWS_KMS","CreationDate":1704067200}}`,
	})
	ecrClient := newECRTestClient(t, map[string]string{
		"AmazonEC2ContainerRegistry_V20150921.DescribeRegistry":     `{"registryId":"123"}`,
		"AmazonEC2ContainerRegistry_V20150921.DescribeRepositories": `{"repositories":[{"repositoryName":"demo","repositoryArn":"arn:aws:ecr:us-east-1:123:repository/demo","registryId":"123","repositoryUri":"123.dkr.ecr.us-east-1.amazonaws.com/demo","createdAt":1704067200}]}`,
		"AmazonEC2ContainerRegistry_V20150921.ListImages":           `{"imageIds":[{"imageDigest":"sha256:abc","imageTag":"v1"}]}`,
	})

	svc := &Service{
		ctx: mcp.ToolsetContext{Redactor: redact.New()},
		eksClient: func(context.Context, string) (*eks.Client, string, error) {
			return eksClient, "us-east-1", nil
		},
		stsClient: func(context.Context, string) (*sts.Client, string, error) {
			return stsClient, "us-east-1", nil
		},
		kmsClient: func(context.Context, string) (*kms.Client, string, error) {
			return kmsClient, "us-east-1", nil
		},
		ecrClient: func(context.Context, string) (*ecr.Client, string, error) {
			return ecrClient, "us-east-1", nil
		},
	}

	result, err := svc.handleDebug(context.Background(), mcp.ToolRequest{Arguments: map[string]any{
		"clusterName":    "demo",
		"includeSts":     true,
		"includeKms":     true,
		"includeEcr":     true,
		"repositoryName": "demo",
		"imageLimit":     1,
	}})
	if err != nil {
		t.Fatalf("debug: %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %#v", result.Data)
	}
	if data["diagnostics"] == nil {
		t.Fatalf("expected diagnostics in response")
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
