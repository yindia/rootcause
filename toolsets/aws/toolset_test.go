package aws

import (
	"testing"

	"rootcause/internal/config"
	"rootcause/internal/kube"
	"rootcause/internal/mcp"
)

func TestToolsetInitAndRegister(t *testing.T) {
	toolset := New()
	if err := toolset.Init(mcp.ToolsetContext{}); err == nil {
		t.Fatalf("expected error for missing clients")
	}
	ctx := mcp.ToolsetContext{Clients: &kube.Clients{}}
	if err := toolset.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}
	cfg := config.DefaultConfig()
	reg := mcp.NewRegistry(&cfg)
	if err := toolset.Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := reg.Get("aws.iam.list_roles"); !ok {
		t.Fatalf("expected aws.iam.list_roles to be registered")
	}
	if _, ok := reg.Get("aws.vpc.list_vpcs"); !ok {
		t.Fatalf("expected aws.vpc.list_vpcs to be registered")
	}
	if _, ok := reg.Get("aws.ec2.list_instances"); !ok {
		t.Fatalf("expected aws.ec2.list_instances to be registered")
	}
	if _, ok := reg.Get("aws.eks.list_clusters"); !ok {
		t.Fatalf("expected aws.eks.list_clusters to be registered")
	}
	if _, ok := reg.Get("aws.ecr.list_repositories"); !ok {
		t.Fatalf("expected aws.ecr.list_repositories to be registered")
	}
	if _, ok := reg.Get("aws.kms.list_keys"); !ok {
		t.Fatalf("expected aws.kms.list_keys to be registered")
	}
	if _, ok := reg.Get("aws.sts.get_caller_identity"); !ok {
		t.Fatalf("expected aws.sts.get_caller_identity to be registered")
	}
}
