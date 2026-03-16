package main

import (
	"context"

	"github.com/spf13/cobra"
)

func executeRoot(ctx context.Context) error {
	return newRootCmd(ctx).Execute()
}

func newRootCmd(ctx context.Context) *cobra.Command {
	cfg := &cliConfig{}
	cmd := &cobra.Command{
		Use:          "rootcause",
		Short:        "RootCause MCP server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(ctx, cfg.toServerOptions(cmd))
		},
	}
	bindRootFlags(cmd, cfg)
	return cmd
}
