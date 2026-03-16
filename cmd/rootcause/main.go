package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"rootcause/pkg/server"

	_ "rootcause/toolsets/aws"
	_ "rootcause/toolsets/helm"
	_ "rootcause/toolsets/istio"
	_ "rootcause/toolsets/k8s"
	_ "rootcause/toolsets/karpenter"
	_ "rootcause/toolsets/linkerd"
	_ "rootcause/toolsets/rootcause"
	_ "rootcause/toolsets/terraform"
)

const version = "0.1.0"

var runServer = server.Run
var exit = os.Exit

func main() {
	ctx := context.Background()

	flags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	kubeconfig := flags.String("kubeconfig", "", "path to kubeconfig")
	contextName := flags.String("context", "", "kubeconfig context")
	toolsets := flags.String("toolsets", "", "comma-separated toolsets to enable")
	configPath := flags.String("config", "", "config file path")
	readOnly := flags.Bool("read-only", false, "disable write operations")
	disableDestructive := flags.Bool("disable-destructive", false, "disable destructive operations")
	transportMode := flags.String("transport", "stdio", "transport mode: stdio|http|sse")
	host := flags.String("host", "127.0.0.1", "host for HTTP/SSE transport")
	port := flags.Int("port", 8000, "port for HTTP/SSE transport")
	path := flags.String("path", "/mcp", "path for HTTP/SSE transport")
	logLevel := flags.String("log-level", "", "log level")

	_ = flags.Parse(os.Args[1:])

	options := server.Options{
		ConfigPath:         *configPath,
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
		Stderr:             os.Stderr,
	}
	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["kubeconfig"] {
		options.Kubeconfig = *kubeconfig
	}
	if set["context"] {
		options.Context = *contextName
	}
	if set["toolsets"] {
		options.Toolsets = parseCSV(*toolsets)
	}
	if set["read-only"] {
		options.ReadOnly = *readOnly
	}
	if set["disable-destructive"] {
		options.DisableDestructive = *disableDestructive
	}
	if set["transport"] {
		options.TransportMode = *transportMode
	}
	if set["host"] {
		options.Host = *host
	}
	if set["port"] {
		options.Port = *port
	}
	if set["path"] {
		options.Path = *path
	}
	if set["log-level"] {
		options.LogLevel = *logLevel
	}

	if err := runServer(ctx, options); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit(1)
	}
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
