package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/route53resolver"
	"github.com/aws/aws-sdk-go-v2/service/sts"

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
	ctx        mcp.ToolsetContext
	iamMu      sync.Mutex
	iamClients map[string]iamClientEntry
	ec2Mu      sync.Mutex
	ec2Clients map[string]ec2ClientEntry
	r53Mu      sync.Mutex
	r53Clients map[string]resolverClientEntry
	asgMu      sync.Mutex
	asgClients map[string]asgClientEntry
	elbMu      sync.Mutex
	elbClients map[string]elbClientEntry
	eksMu      sync.Mutex
	eksClients map[string]eksClientEntry
	ecrMu      sync.Mutex
	ecrClients map[string]ecrClientEntry
	kmsMu      sync.Mutex
	kmsClients map[string]kmsClientEntry
	stsMu      sync.Mutex
	stsClients map[string]stsClientEntry
}

type iamClientEntry struct {
	client *iam.Client
	region string
}

type ec2ClientEntry struct {
	client *ec2.Client
	region string
}

type resolverClientEntry struct {
	client *route53resolver.Client
	region string
}

type asgClientEntry struct {
	client *autoscaling.Client
	region string
}

type elbClientEntry struct {
	client *elasticloadbalancingv2.Client
	region string
}

type eksClientEntry struct {
	client *eks.Client
	region string
}

type ecrClientEntry struct {
	client *ecr.Client
	region string
}

type kmsClientEntry struct {
	client *kms.Client
	region string
}

type stsClientEntry struct {
	client *sts.Client
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

func (t *Toolset) Init(ctx mcp.ToolsetContext) error {
	if ctx.Clients == nil {
		return errors.New("missing kube clients")
	}
	t.ctx = ctx
	t.iamClients = map[string]iamClientEntry{}
	t.ec2Clients = map[string]ec2ClientEntry{}
	t.r53Clients = map[string]resolverClientEntry{}
	t.asgClients = map[string]asgClientEntry{}
	t.elbClients = map[string]elbClientEntry{}
	t.eksClients = map[string]eksClientEntry{}
	t.ecrClients = map[string]ecrClientEntry{}
	t.kmsClients = map[string]kmsClientEntry{}
	t.stsClients = map[string]stsClientEntry{}
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

func (t *Toolset) iamClient(ctx context.Context, region string) (*iam.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.iamMu.Lock()
	if entry, ok := t.iamClients[cacheKey]; ok {
		t.iamMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.iamMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := iam.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.iamMu.Lock()
	t.iamClients[cacheKey] = iamClientEntry{client: client, region: usedRegion}
	t.iamMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) ec2Client(ctx context.Context, region string) (*ec2.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.ec2Mu.Lock()
	if entry, ok := t.ec2Clients[cacheKey]; ok {
		t.ec2Mu.Unlock()
		return entry.client, entry.region, nil
	}
	t.ec2Mu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := ec2.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.ec2Mu.Lock()
	t.ec2Clients[cacheKey] = ec2ClientEntry{client: client, region: usedRegion}
	t.ec2Mu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) resolverClient(ctx context.Context, region string) (*route53resolver.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.r53Mu.Lock()
	if entry, ok := t.r53Clients[cacheKey]; ok {
		t.r53Mu.Unlock()
		return entry.client, entry.region, nil
	}
	t.r53Mu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := route53resolver.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.r53Mu.Lock()
	t.r53Clients[cacheKey] = resolverClientEntry{client: client, region: usedRegion}
	t.r53Mu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) asgClient(ctx context.Context, region string) (*autoscaling.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.asgMu.Lock()
	if entry, ok := t.asgClients[cacheKey]; ok {
		t.asgMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.asgMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := autoscaling.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.asgMu.Lock()
	t.asgClients[cacheKey] = asgClientEntry{client: client, region: usedRegion}
	t.asgMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) elbClient(ctx context.Context, region string) (*elasticloadbalancingv2.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.elbMu.Lock()
	if entry, ok := t.elbClients[cacheKey]; ok {
		t.elbMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.elbMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.elbMu.Lock()
	t.elbClients[cacheKey] = elbClientEntry{client: client, region: usedRegion}
	t.elbMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) eksClient(ctx context.Context, region string) (*eks.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.eksMu.Lock()
	if entry, ok := t.eksClients[cacheKey]; ok {
		t.eksMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.eksMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := eks.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.eksMu.Lock()
	t.eksClients[cacheKey] = eksClientEntry{client: client, region: usedRegion}
	t.eksMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) ecrClient(ctx context.Context, region string) (*ecr.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.ecrMu.Lock()
	if entry, ok := t.ecrClients[cacheKey]; ok {
		t.ecrMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.ecrMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := ecr.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.ecrMu.Lock()
	t.ecrClients[cacheKey] = ecrClientEntry{client: client, region: usedRegion}
	t.ecrMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) kmsClient(ctx context.Context, region string) (*kms.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.kmsMu.Lock()
	if entry, ok := t.kmsClients[cacheKey]; ok {
		t.kmsMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.kmsMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := kms.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.kmsMu.Lock()
	t.kmsClients[cacheKey] = kmsClientEntry{client: client, region: usedRegion}
	t.kmsMu.Unlock()
	return client, usedRegion, nil
}

func (t *Toolset) stsClient(ctx context.Context, region string) (*sts.Client, string, error) {
	cacheKey := t.clientCacheKey(region)
	t.stsMu.Lock()
	if entry, ok := t.stsClients[cacheKey]; ok {
		t.stsMu.Unlock()
		return entry.client, entry.region, nil
	}
	t.stsMu.Unlock()

	cfg, err := awslib.LoadConfig(ctx, region)
	if err != nil {
		return nil, "", err
	}
	client := sts.NewFromConfig(cfg)
	usedRegion := strings.TrimSpace(cfg.Region)
	t.stsMu.Lock()
	t.stsClients[cacheKey] = stsClientEntry{client: client, region: usedRegion}
	t.stsMu.Unlock()
	return client, usedRegion, nil
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
