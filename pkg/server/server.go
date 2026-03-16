package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"rootcause/internal/audit"
	"rootcause/internal/cache"
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
	Transport          sdkmcp.Transport
	TransportMode      string
	Host               string
	Port               int
	Path               string
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
	if strings.TrimSpace(opts.TransportMode) != "" {
		value := strings.TrimSpace(opts.TransportMode)
		overrides.TransportMode = &value
	}
	if strings.TrimSpace(opts.Host) != "" {
		value := strings.TrimSpace(opts.Host)
		overrides.TransportHost = &value
	}
	if opts.Port > 0 {
		value := opts.Port
		overrides.TransportPort = &value
	}
	if strings.TrimSpace(opts.Path) != "" {
		value := strings.TrimSpace(opts.Path)
		overrides.TransportPath = &value
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
	promptNames, err := rcmcp.RegisterSDKPrompts(server, toolCtx)
	if err != nil {
		return fmt.Errorf("prompt registration failed: %w", err)
	}
	resourceURIs, resourceTemplates, err := rcmcp.RegisterSDKResources(server, toolCtx)
	if err != nil {
		return fmt.Errorf("resource registration failed: %w", err)
	}

	reloadCh := make(chan os.Signal, 1)
	notifyReload(reloadCh)
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
			if len(promptNames) > 0 {
				server.RemovePrompts(promptNames...)
			}
			if len(resourceURIs) > 0 {
				server.RemoveResources(resourceURIs...)
			}
			if len(resourceTemplates) > 0 {
				server.RemoveResourceTemplates(resourceTemplates...)
			}
			toolNames, err = rcmcp.RegisterSDKTools(server, reg, toolCtx)
			if err != nil {
				fmt.Fprintf(errOut, "tool registration failed: %v\n", err)
				continue
			}
			promptNames, err = rcmcp.RegisterSDKPrompts(server, toolCtx)
			if err != nil {
				fmt.Fprintf(errOut, "prompt registration failed: %v\n", err)
				continue
			}
			resourceURIs, resourceTemplates, err = rcmcp.RegisterSDKResources(server, toolCtx)
			if err != nil {
				fmt.Fprintf(errOut, "resource registration failed: %v\n", err)
				continue
			}
		}
	}()

	mode := strings.ToLower(strings.TrimSpace(cfg.Transport.Mode))
	if mode == "" {
		mode = "stdio"
	}
	if mode == "stdio" {
		transport := opts.Transport
		if transport == nil {
			transport = &sdkmcp.StdioTransport{}
		}
		if err := server.Run(ctx, transport); err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	}

	host := strings.TrimSpace(cfg.Transport.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Transport.Port
	if port <= 0 {
		port = 8000
	}
	path := strings.TrimSpace(cfg.Transport.Path)
	if path == "" {
		path = "/mcp"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	mux := http.NewServeMux()
	switch mode {
	case "http":
		handler := sdkmcp.NewStreamableHTTPHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)
		mux.Handle(path, handler)
	case "sse":
		handler := sdkmcp.NewSSEHandler(func(*http.Request) *sdkmcp.Server { return server }, nil)
		mux.Handle(path, handler)
	default:
		return fmt.Errorf("unsupported transport mode: %s", mode)
	}
	httpServer := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	err = httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
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
	cacheStore := cache.NewStore()
	callGraph := rcmcp.NewCallGraph()
	reg := rcmcp.NewRegistry(&cfg)

	toolCtx := rcmcp.ToolContext{
		Config:    &cfg,
		Clients:   clients,
		Policy:    authorizer,
		Evidence:  evidenceCollector,
		Renderer:  renderer,
		Redactor:  redactor,
		Audit:     auditLogger,
		Services:  serviceRegistry,
		Cache:     cacheStore,
		CallGraph: callGraph,
		Registry:  reg,
	}
	toolCtx.Invoker = rcmcp.NewToolInvoker(reg, toolCtx)
	toolsetCtx := rcmcp.ToolsetContext(toolCtx)

	for _, id := range effectiveToolsets(cfg.Toolsets) {
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
	if err := rcmcp.ValidateToolDependencies(reg, rcmcp.RequiredToolDependencies()); err != nil {
		return rcmcp.ToolContext{}, nil, err
	}

	return toolCtx, reg, nil
}

func effectiveToolsets(toolsets []string) []string {
	out := append([]string{}, toolsets...)
	if !browserEnabledFromEnv() {
		return out
	}
	for _, id := range out {
		if id == "browser" {
			return out
		}
	}
	return append(out, "browser")
}

func browserEnabledFromEnv() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("MCP_BROWSER_ENABLED")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
