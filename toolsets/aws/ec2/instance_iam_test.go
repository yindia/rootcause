package awsec2

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"rootcause/internal/mcp"
	"rootcause/internal/redact"
)

func TestEC2GetInstanceIAM(t *testing.T) {
	ctx := mcp.ToolsetContext{Redactor: redact.New()}

	noIAM := &Service{
		ctx:       ctx,
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) { return nil, "", nil },
	}
	if _, err := noIAM.handleGetInstanceIAM(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}}); err == nil {
		t.Fatalf("expected iam client error")
	}

	ec2NoProfile := newEC2TestClient(t, map[string]string{
		"DescribeInstances": `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-1</instanceId>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`,
	})
	svcNoProfile := &Service{
		ctx: ctx,
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2NoProfile, "us-east-1", nil
		},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return nil, "", nil
		},
	}
	result, err := svcNoProfile.handleGetInstanceIAM(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-1"}})
	if err != nil {
		t.Fatalf("expected no profile result, got %v", err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["iam"] == nil {
		t.Fatalf("expected iam message, got %#v", result.Data)
	}

	ec2WithProfile := newEC2TestClient(t, map[string]string{
		"DescribeInstances": `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-2</instanceId>
          <iamInstanceProfile>
            <arn>arn:aws:iam::123:instance-profile/profile</arn>
            <id>ip-1</id>
          </iamInstanceProfile>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`,
	})
	iamClient := newIAMTestClient(t, map[string]string{
		"GetInstanceProfile": `<GetInstanceProfileResponse xmlns="https://iam.amazonaws.com/doc/2010-05-08/">
  <GetInstanceProfileResult>
    <InstanceProfile>
      <Path>/</Path>
      <InstanceProfileName>profile</InstanceProfileName>
      <InstanceProfileId>IPID</InstanceProfileId>
      <Arn>arn:aws:iam::123:instance-profile/profile</Arn>
      <CreateDate>2024-01-01T00:00:00Z</CreateDate>
    </InstanceProfile>
  </GetInstanceProfileResult>
</GetInstanceProfileResponse>`,
	})
	svcProfile := &Service{
		ctx: ctx,
		ec2Client: func(context.Context, string) (*ec2.Client, string, error) {
			return ec2WithProfile, "us-east-1", nil
		},
		iamClient: func(context.Context, string) (*iam.Client, string, error) {
			return iamClient, "us-east-1", nil
		},
	}
	if _, err := svcProfile.handleGetInstanceIAM(context.Background(), mcp.ToolRequest{Arguments: map[string]any{"instanceId": "i-2"}}); err != nil {
		t.Fatalf("expected profile result, got %v", err)
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
