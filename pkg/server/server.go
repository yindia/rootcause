package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

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
	// Transport is an optional injection point for tests. When nil, stdio
	// is used. Rootcause only speaks stdio — remote/network access should
	// be fronted by a reverse proxy that exposes the stdio transport.
	Transport sdkmcp.Transport
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

	toolCtx, _, err := buildRuntime(cfg, errOut, nil)
	if err != nil {
		return fmt.Errorf("init failed: %w", err)
	}
	invoker := toolCtx.Invoker
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "rootcause", Version: opts.Version}, nil)
	toolNames, err := rcmcp.RegisterSDKTools(server, invoker)
	if err != nil {
		return fmt.Errorf("tool registration failed: %w", err)
	}
	// Prompt and resource registration failures are non-fatal — they're often
	// caused by user-edited files (custom prompts) or cluster-state probes
	// (resources) that shouldn't prevent the server from serving its tools.
	// Matches the (newly) lenient reload behavior in the goroutine below.
	promptNames, err := rcmcp.RegisterSDKPrompts(server, toolCtx)
	if err != nil {
		fmt.Fprintf(errOut, "rootcause: prompt registration failed at startup, continuing without prompts: %v\n", err)
		promptNames = nil
	}
	resourceURIs, resourceTemplates, err := rcmcp.RegisterSDKResources(server, toolCtx)
	if err != nil {
		fmt.Fprintf(errOut, "rootcause: resource registration failed at startup, continuing without resources: %v\n", err)
		resourceURIs, resourceTemplates = nil, nil
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
			newCtx, newReg, err := buildRuntime(cfg, errOut, invoker)
			if err != nil {
				fmt.Fprintf(errOut, "reload init failed: %v\n", err)
				continue
			}
			newNames := newReg.Names()
			toRemove, toAdd := diffToolNames(toolNames, newNames)
			if len(toRemove) > 0 {
				server.RemoveTools(toRemove...)
			}
			for _, name := range toAdd {
				if spec, ok := newReg.Get(name); ok {
					rcmcp.AddSDKTool(server, spec, invoker)
				}
			}
			toolNames = newNames

			// Prompts: probe the load before removing the live set, so a bad
			// prompt edit on reload never leaves the server with zero prompts.
			if _, probeErr := rcmcp.LoadPromptSpecsForCLI(newCtx); probeErr != nil {
				fmt.Fprintf(errOut, "prompt reload skipped, keeping previous prompts: %v\n", probeErr)
			} else {
				if len(promptNames) > 0 {
					server.RemovePrompts(promptNames...)
				}
				if names, perr := rcmcp.RegisterSDKPrompts(server, newCtx); perr != nil {
					fmt.Fprintf(errOut, "prompt registration failed: %v\n", perr)
				} else {
					promptNames = names
				}
			}

			// Resources: remove + re-register independently of prompts so a
			// failure in one surface never wipes the other.
			if len(resourceURIs) > 0 {
				server.RemoveResources(resourceURIs...)
			}
			if len(resourceTemplates) > 0 {
				server.RemoveResourceTemplates(resourceTemplates...)
			}
			if uris, tmpls, rerr := rcmcp.RegisterSDKResources(server, newCtx); rerr != nil {
				fmt.Fprintf(errOut, "resource registration failed: %v\n", rerr)
				resourceURIs, resourceTemplates = nil, nil
			} else {
				resourceURIs, resourceTemplates = uris, tmpls
			}
		}
	}()

	transport := opts.Transport
	if transport == nil {
		transport = &sdkmcp.StdioTransport{}
	}
	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func buildRuntime(cfg config.Config, errOut io.Writer, existingInvoker *rcmcp.ToolInvoker) (rcmcp.ToolContext, *rcmcp.ToolRegistry, error) {
	// A missing/unreachable kubeconfig is non-fatal: cloud-only toolsets (gcp,
	// aws, terraform) and rootcause can still start. Toolsets that genuinely
	// need a cluster (k8s, helm, istio, karpenter, linkerd) fail their own Init
	// with a clear "missing kube clients" error when they're enabled.
	clients, err := kube.NewClients(kube.Config{
		Kubeconfig: cfg.Kubeconfig,
		Context:    cfg.Context,
	})
	if err != nil {
		fmt.Fprintf(errOut, "rootcause: kubeconfig unavailable, k8s-dependent toolsets will be disabled: %v\n", err)
		clients = nil
	}
	authorizer := policy.NewAuthorizer()
	redactor := redact.New()
	renderer := render.NewRenderer()
	evidenceCollector := evidence.NewCollector(clients)
	auditLogger := audit.NewLogger(errOut)
	cacheStore := cache.NewStore()
	callGraph := rcmcp.NewCallGraph(cfg.Limits.MaxCallGraph)
	reg := rcmcp.NewRegistry(&cfg)

	toolCtx := rcmcp.ToolContext{
		Config:    &cfg,
		Clients:   clients,
		Policy:    authorizer,
		Evidence:  evidenceCollector,
		Renderer:  renderer,
		Redactor:  redactor,
		Audit:     auditLogger,
		Cache:     cacheStore,
		CallGraph: callGraph,
		Registry:  reg,
	}
	if existingInvoker != nil {
		toolCtx.Invoker = existingInvoker
	} else {
		toolCtx.Invoker = rcmcp.NewToolInvoker(reg, toolCtx)
	}

	for _, id := range effectiveToolsets(cfg.Toolsets) {
		factory, ok := rcmcp.ToolsetFactoryFor(id)
		if !ok {
			return rcmcp.ToolContext{}, nil, fmt.Errorf("unknown toolset: %s", id)
		}
		toolset := factory()
		if err := toolset.Init(toolCtx); err != nil {
			return rcmcp.ToolContext{}, nil, err
		}
		if err := toolset.Register(reg); err != nil {
			return rcmcp.ToolContext{}, nil, err
		}
	}
	if err := rcmcp.ValidateToolDependencies(reg, rcmcp.RequiredToolDependencies()); err != nil {
		return rcmcp.ToolContext{}, nil, err
	}

	if existingInvoker != nil {
		existingInvoker.Swap(reg, toolCtx)
	}

	return toolCtx, reg, nil
}

func diffToolNames(oldNames, newNames []string) (toRemove, toAdd []string) {
	oldSet := make(map[string]struct{}, len(oldNames))
	for _, n := range oldNames {
		oldSet[n] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newNames))
	for _, n := range newNames {
		newSet[n] = struct{}{}
	}
	for _, n := range oldNames {
		if _, ok := newSet[n]; !ok {
			toRemove = append(toRemove, n)
		}
	}
	for _, n := range newNames {
		if _, ok := oldSet[n]; !ok {
			toAdd = append(toAdd, n)
		}
	}
	return toRemove, toAdd
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
