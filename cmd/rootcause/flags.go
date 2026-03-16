package main

import "github.com/spf13/cobra"

type cliConfig struct {
	kubeconfig         string
	contextName        string
	toolsets           string
	configPath         string
	readOnly           bool
	disableDestructive bool
	transportMode      string
	host               string
	port               int
	path               string
	logLevel           string
}

func bindRootFlags(cmd *cobra.Command, cfg *cliConfig) {
	flags := cmd.Flags()
	flags.StringVar(&cfg.kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flags.StringVar(&cfg.contextName, "context", "", "kubeconfig context")
	flags.StringVar(&cfg.toolsets, "toolsets", "", "comma-separated toolsets to enable")
	flags.StringVar(&cfg.configPath, "config", "", "config file path")
	flags.BoolVar(&cfg.readOnly, "read-only", false, "disable write operations")
	flags.BoolVar(&cfg.disableDestructive, "disable-destructive", false, "disable destructive operations")
	flags.StringVar(&cfg.transportMode, "transport", "stdio", "transport mode: stdio|http|sse")
	flags.StringVar(&cfg.host, "host", "127.0.0.1", "host for HTTP/SSE transport")
	flags.IntVar(&cfg.port, "port", 8000, "port for HTTP/SSE transport")
	flags.StringVar(&cfg.path, "path", "/mcp", "path for HTTP/SSE transport")
	flags.StringVar(&cfg.logLevel, "log-level", "", "log level")
}
