package main

import (
	"os"

	"github.com/spf13/cobra"

	"rootcause/pkg/server"
)

func (cfg *cliConfig) toServerOptions(cmd *cobra.Command) server.Options {
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
		Stderr:             os.Stderr,
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
