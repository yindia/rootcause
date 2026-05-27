package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sdkaws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/sync/singleflight"

	awslib "rootcause/internal/aws"
	"rootcause/internal/mcp"
	awsec2 "rootcause/toolsets/aws/ec2"
	awsecr "rootcause/toolsets/aws/ecr"
	awseks "rootcause/toolsets/aws/eks"
	awsiam "rootcause/toolsets/aws/iam"
	awskms "rootcause/toolsets/aws/kms"
	awssts "rootcause/toolsets/aws/sts"
	awsvpc "rootcause/toolsets/aws/vpc"
)

type Toolset struct {
	ctx   mcp.ToolContext
	cache sync.Map
	sf    singleflight.Group
}

type clientEntry struct {
	client any
	region string
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("aws", func() mcp.Toolset {
		return New()
	})
}

func (t *Toolset) ID() string {
	return "aws"
}

func (t *Toolset) Version() string {
	return "0.1.0"
}

func (t *Toolset) Init(ctx mcp.ToolContext) error {
	// The AWS toolset talks only to AWS APIs; it does not use kube clients, so
	// it can initialize without a cluster.
	t.ctx = ctx
	t.cache = sync.Map{}
	t.sf = singleflight.Group{}
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	for _, tool := range awsiam.ToolSpecs(t.ctx, t.ID(), t.iamClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awsvpc.ToolSpecs(t.ctx, t.ID(), t.ec2Client, t.resolverClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awsec2.ToolSpecs(t.ctx, t.ID(), t.ec2Client, t.asgClient, t.elbClient, t.iamClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awseks.ToolSpecs(t.ctx, t.ID(), t.eksClient, t.ec2Client, t.asgClient, t.ecrClient, t.kmsClient, t.stsClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awsecr.ToolSpecs(t.ctx, t.ID(), t.ecrClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awskms.ToolSpecs(t.ctx, t.ID(), t.kmsClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range awssts.ToolSpecs(t.ctx, t.ID(), t.stsClient) {
		tool = t.wrapListCache(tool)
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}

func (t *Toolset) loadClient(ctx context.Context, service, region string, build func(sdkaws.Config) any) (any, string, error) {
	cacheKey := t.clientCacheKey(region)
	fullKey := service + "|" + cacheKey
	if raw, ok := t.cache.Load(fullKey); ok {
		entry := raw.(*clientEntry)
		return entry.client, entry.region, nil
	}
	resolved, err, _ := t.sf.Do(fullKey, func() (any, error) {
		if raw, ok := t.cache.Load(fullKey); ok {
			return raw, nil
		}
		cfg, err := awslib.LoadConfig(ctx, region)
		if err != nil {
			return nil, err
		}
		entry := &clientEntry{client: build(cfg), region: strings.TrimSpace(cfg.Region)}
		t.cache.Store(fullKey, entry)
		return entry, nil
	})
	if err != nil {
		return nil, "", err
	}
	entry := resolved.(*clientEntry)
	return entry.client, entry.region, nil
}

func (t *Toolset) iamClient(ctx context.Context, region string) (*iam.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "iam", region, func(cfg sdkaws.Config) any { return iam.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*iam.Client), used, nil
}

func (t *Toolset) ec2Client(ctx context.Context, region string) (*ec2.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "ec2", region, func(cfg sdkaws.Config) any { return ec2.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*ec2.Client), used, nil
}

func (t *Toolset) resolverClient(ctx context.Context, region string) (*route53resolver.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "route53resolver", region, func(cfg sdkaws.Config) any { return route53resolver.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*route53resolver.Client), used, nil
}

func (t *Toolset) asgClient(ctx context.Context, region string) (*autoscaling.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "autoscaling", region, func(cfg sdkaws.Config) any { return autoscaling.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*autoscaling.Client), used, nil
}

func (t *Toolset) elbClient(ctx context.Context, region string) (*elasticloadbalancingv2.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "elbv2", region, func(cfg sdkaws.Config) any { return elasticloadbalancingv2.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*elasticloadbalancingv2.Client), used, nil
}

func (t *Toolset) eksClient(ctx context.Context, region string) (*eks.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "eks", region, func(cfg sdkaws.Config) any { return eks.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*eks.Client), used, nil
}

func (t *Toolset) ecrClient(ctx context.Context, region string) (*ecr.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "ecr", region, func(cfg sdkaws.Config) any { return ecr.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*ecr.Client), used, nil
}

func (t *Toolset) kmsClient(ctx context.Context, region string) (*kms.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "kms", region, func(cfg sdkaws.Config) any { return kms.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*kms.Client), used, nil
}

func (t *Toolset) stsClient(ctx context.Context, region string) (*sts.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "sts", region, func(cfg sdkaws.Config) any { return sts.NewFromConfig(cfg) })
	if err != nil {
		return nil, "", err
	}
	return raw.(*sts.Client), used, nil
}

func (t *Toolset) clientCacheKey(region string) string {
	regionKey := awslib.ResolveRegion(region)
	profile := awslib.ResolveProfile()
	cacheKey := regionKey
	if cacheKey == "" {
		cacheKey = "default"
	}
	if profile != "" {
		cacheKey = strings.TrimSpace(profile) + "|" + cacheKey
	}
	return cacheKey
}
