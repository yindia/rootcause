package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/audit"
	"rootcause/internal/config"
	"rootcause/internal/evidence"
	"rootcause/internal/kube"
	rcmcp "rootcause/internal/mcp"
	"rootcause/internal/policy"
	"rootcause/internal/redact"
	"rootcause/internal/render"
)

type Options struct {
	ConfigPath         string
	Kubeconfig         string
	Context            string
	Toolsets           []string
	ReadOnly           bool
	DisableDestructive bool
	LogLevel           string
	Version            string
	Stderr             io.Writer
}

func Run(ctx context.Context, opts Options) error {
	errOut := opts.Stderr
	if errOut == nil {
		errOut = os.Stderr
	}
	configPath := opts.ConfigPath
	if configPath == "" {
		if env := os.Getenv("ROOTCAUSE_CONFIG"); env != "" {
			configPath = env
		}
	}
	overrides := config.Overrides{}
	if opts.Kubeconfig != "" {
		overrides.Kubeconfig = &opts.Kubeconfig
	}
	if opts.Context != "" {
		overrides.Context = &opts.Context
	}
	if len(opts.Toolsets) > 0 {
		overrides.Toolsets = &opts.Toolsets
	}
	if opts.ReadOnly {
		overrides.ReadOnly = &opts.ReadOnly
	}
	if opts.DisableDestructive {
		overrides.DisableDestructive = &opts.DisableDestructive
	}
	if opts.LogLevel != "" {
		overrides.LogLevel = &opts.LogLevel
	}

	cfg, err := config.Load(configPath, "", overrides)
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	toolCtx, reg, err := buildRuntime(cfg, errOut)
	if err != nil {
		return fmt.Errorf("init failed: %w", err)
	}
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rootcause", Version: opts.Version}, nil)
	toolNames, err := rcmcp.RegisterSDKTools(server, reg, toolCtx)
	if err != nil {
		return fmt.Errorf("tool registration failed: %w", err)
	}

	reloadCh := make(chan os.Signal, 1)
	signal.Notify(reloadCh, syscall.SIGHUP)
	go func() {
		for range reloadCh {
			cfg, err := config.Load(configPath, "", overrides)
			if err != nil {
				fmt.Fprintf(errOut, "config reload failed: %v\n", err)
				continue
			}
			toolCtx, reg, err := buildRuntime(cfg, errOut)
			if err != nil {
				fmt.Fprintf(errOut, "reload init failed: %v\n", err)
				continue
			}
			if len(toolNames) > 0 {
				server.RemoveTools(toolNames...)
			}
			toolNames, err = rcmcp.RegisterSDKTools(server, reg, toolCtx)
			if err != nil {
				fmt.Fprintf(errOut, "tool registration failed: %v\n", err)
				continue
			}
		}
	}()

	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func buildRuntime(cfg config.Config, errOut io.Writer) (rcmcp.ToolContext, *rcmcp.ToolRegistry, error) {
	clients, err := kube.NewClients(kube.Config{
		Kubeconfig: cfg.Kubeconfig,
		Context:    cfg.Context,
	})
	if err != nil {
		return rcmcp.ToolContext{}, nil, err
	}
	authorizer := policy.NewAuthorizer()
	redactor := redact.New()
	renderer := render.NewRenderer()
	evidenceCollector := evidence.NewCollector(clients)
	auditLogger := audit.NewLogger(errOut)
	serviceRegistry := rcmcp.NewServiceRegistry()
	reg := rcmcp.NewRegistry(&cfg)

	toolCtx := rcmcp.ToolContext{
		Config:   &cfg,
		Clients:  clients,
		Policy:   authorizer,
		Evidence: evidenceCollector,
		Renderer: renderer,
		Redactor: redactor,
		Audit:    auditLogger,
		Services: serviceRegistry,
		Registry: reg,
	}
	toolCtx.Invoker = rcmcp.NewToolInvoker(reg, toolCtx)
	toolsetCtx := rcmcp.ToolsetContext(toolCtx)

	for _, id := range cfg.Toolsets {
		factory, ok := rcmcp.ToolsetFactoryFor(id)
		if !ok {
			return rcmcp.ToolContext{}, nil, fmt.Errorf("unknown toolset: %s", id)
		}
		toolset := factory()
		if err := toolset.Init(toolsetCtx); err != nil {
			return rcmcp.ToolContext{}, nil, err
		}
		if err := toolset.Register(reg); err != nil {
			return rcmcp.ToolContext{}, nil, err
		}
	}

	return toolCtx, reg, nil
}
