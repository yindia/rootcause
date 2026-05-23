package gcp

import (
	"context"
	"errors"
	"fmt"
	"sync"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/logging/logadmin"
	"golang.org/x/sync/singleflight"
	"google.golang.org/api/option"

	gcpcfg "rootcause/internal/gcp"
	"rootcause/internal/mcp"
	gcplogging "rootcause/toolsets/gcp/logging"
	gcpmonitoring "rootcause/toolsets/gcp/monitoring"
)

type Toolset struct {
	ctx   mcp.ToolContext
	cache sync.Map
	sf    singleflight.Group
}

type clientEntry struct {
	client any
	project string
}

func New() *Toolset {
	return &Toolset{}
}

func init() {
	mcp.MustRegisterToolset("gcp", func() mcp.Toolset { return New() })
}

func (t *Toolset) ID() string      { return "gcp" }
func (t *Toolset) Version() string { return "0.1.0" }

func (t *Toolset) Init(ctx mcp.ToolContext) error {
	if ctx.Clients == nil {
		return errors.New("missing kube clients")
	}
	t.ctx = ctx
	t.cache = sync.Map{}
	t.sf = singleflight.Group{}
	return nil
}

func (t *Toolset) Register(reg mcp.Registry) error {
	for _, tool := range gcpmonitoring.ToolSpecs(t.ctx, t.ID(), t.queryClient, t.metricClient, t.slmClient) {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	for _, tool := range gcplogging.ToolSpecs(t.ctx, t.ID(), t.logClient) {
		if err := reg.Add(tool); err != nil {
			return fmt.Errorf("register %s: %w", tool.Name, err)
		}
	}
	return nil
}

func (t *Toolset) loadClient(ctx context.Context, service, projectExplicit string, build func(ctx context.Context, project string, opts []option.ClientOption) (any, error)) (any, string, error) {
	project := gcpcfg.ResolveProjectWithKubeconfig(projectExplicit)
	if project == "" {
		return nil, "", errors.New("gcp project id is required (set GOOGLE_CLOUD_PROJECT, pass projectId, or use a GKE kubeconfig context)")
	}
	fullKey := service + "|" + project
	if raw, ok := t.cache.Load(fullKey); ok {
		entry := raw.(*clientEntry)
		return entry.client, entry.project, nil
	}
	resolved, err, _ := t.sf.Do(fullKey, func() (any, error) {
		if raw, ok := t.cache.Load(fullKey); ok {
			return raw, nil
		}
		opts := []option.ClientOption{}
		if credsFile := gcpcfg.CredentialsFile(); credsFile != "" {
			opts = append(opts, option.WithCredentialsFile(credsFile))
		}
		client, err := build(ctx, project, opts)
		if err != nil {
			return nil, err
		}
		entry := &clientEntry{client: client, project: project}
		t.cache.Store(fullKey, entry)
		return entry, nil
	})
	if err != nil {
		return nil, "", err
	}
	entry := resolved.(*clientEntry)
	return entry.client, entry.project, nil
}

func (t *Toolset) queryClient(ctx context.Context, project string) (*monitoring.QueryClient, string, error) {
	raw, used, err := t.loadClient(ctx, "monitoring.query", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewQueryClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.QueryClient), used, nil
}

func (t *Toolset) metricClient(ctx context.Context, project string) (*monitoring.MetricClient, string, error) {
	raw, used, err := t.loadClient(ctx, "monitoring.metric", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewMetricClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.MetricClient), used, nil
}

func (t *Toolset) slmClient(ctx context.Context, project string) (*monitoring.ServiceMonitoringClient, string, error) {
	raw, used, err := t.loadClient(ctx, "monitoring.servicemonitoring", project, func(ctx context.Context, _ string, opts []option.ClientOption) (any, error) {
		return monitoring.NewServiceMonitoringClient(ctx, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*monitoring.ServiceMonitoringClient), used, nil
}

func (t *Toolset) logClient(ctx context.Context, project string) (*logadmin.Client, string, error) {
	raw, used, err := t.loadClient(ctx, "logging.admin", project, func(ctx context.Context, project string, opts []option.ClientOption) (any, error) {
		return logadmin.NewClient(ctx, project, opts...)
	})
	if err != nil {
		return nil, "", err
	}
	return raw.(*logadmin.Client), used, nil
}
