package cli

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"rootcause/pkg/server"
)

type RunServerFunc func(ctx context.Context, opts server.Options) error

type config struct {
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

func Execute(ctx context.Context, args []string, run RunServerFunc, version string, stderr io.Writer) error {
	if run == nil {
		run = server.Run
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	cfg := &config{}
	cmd := newRootCmd(ctx, run, version, stderr, cfg)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newRootCmd(ctx context.Context, run RunServerFunc, version string, stderr io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "rootcause",
		Short:        "RootCause MCP server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(ctx, cfg.toServerOptions(cmd, version, stderr))
		},
	}
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
	cmd.AddCommand(newSyncSkillsCmd(stderr))
	return cmd
}

func (cfg *config) toServerOptions(cmd *cobra.Command, version string, stderr io.Writer) server.Options {
	options := server.Options{
		ConfigPath:         cfg.configPath,
		Kubeconfig:         "",
		Context:            "",
		Toolsets:           nil,
		ReadOnly:           false,
		DisableDestructive: false,
		TransportMode:      "",
		Host:               "",
		Port:               0,
		Path:               "",
		LogLevel:           "",
		Version:            version,
		Stderr:             stderr,
	}
	if cmd.Flags().Lookup("kubeconfig").Changed {
		options.Kubeconfig = cfg.kubeconfig
	}
	if cmd.Flags().Lookup("context").Changed {
		options.Context = cfg.contextName
	}
	if cmd.Flags().Lookup("toolsets").Changed {
		options.Toolsets = parseCSV(cfg.toolsets)
	}
	if cmd.Flags().Lookup("read-only").Changed {
		options.ReadOnly = cfg.readOnly
	}
	if cmd.Flags().Lookup("disable-destructive").Changed {
		options.DisableDestructive = cfg.disableDestructive
	}
	if cmd.Flags().Lookup("transport").Changed {
		options.TransportMode = cfg.transportMode
	}
	if cmd.Flags().Lookup("host").Changed {
		options.Host = cfg.host
	}
	if cmd.Flags().Lookup("port").Changed {
		options.Port = cfg.port
	}
	if cmd.Flags().Lookup("path").Changed {
		options.Path = cfg.path
	}
	if cmd.Flags().Lookup("log-level").Changed {
		options.LogLevel = cfg.logLevel
	}
	return options
}

func parseCSV(input string) []string {
	if input == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	var out []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
