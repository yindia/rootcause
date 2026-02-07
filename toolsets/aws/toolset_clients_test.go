package aws

import (
	"context"
	"testing"

	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestToolsetClientCaching(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-west-2")

	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{Clients: &kube.Clients{}}); err != nil {
		t.Fatalf("init toolset: %v", err)
	}

	iam1, region1, err := toolset.iamClient(context.Background(), "")
	if err != nil || iam1 == nil {
		t.Fatalf("iam client: %v", err)
	}
	iam2, region2, err := toolset.iamClient(context.Background(), "")
	if err != nil || iam2 == nil {
		t.Fatalf("iam client (cached): %v", err)
	}
	if iam1 != iam2 || region1 != region2 {
		t.Fatalf("expected cached iam client")
	}

	ec1, _, err := toolset.ec2Client(context.Background(), "")
	if err != nil || ec1 == nil {
		t.Fatalf("ec2 client: %v", err)
	}
	_, _, err = toolset.resolverClient(context.Background(), "")
	if err != nil {
		t.Fatalf("resolver client: %v", err)
	}
	_, _, err = toolset.asgClient(context.Background(), "")
	if err != nil {
		t.Fatalf("asg client: %v", err)
	}
	_, _, err = toolset.elbClient(context.Background(), "")
	if err != nil {
		t.Fatalf("elb client: %v", err)
	}
	_, _, err = toolset.eksClient(context.Background(), "")
	if err != nil {
		t.Fatalf("eks client: %v", err)
	}

	ecOther, _, err := toolset.ec2Client(context.Background(), "us-east-1")
	if err != nil || ecOther == nil {
		t.Fatalf("ec2 client other region: %v", err)
	}
	if ecOther == ec1 {
		t.Fatalf("expected different ec2 client for other region")
	}
}
