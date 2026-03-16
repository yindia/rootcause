package main

import (
	"context"
	"fmt"
	"os"

	"rootcause/cmd/rootcause/cli"
	"rootcause/pkg/server"

	_ "rootcause/toolsets/aws"
	_ "rootcause/toolsets/browser"
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
	if err := executeRoot(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit(1)
	}
}

func executeRoot(ctx context.Context) error {
	return cli.Execute(ctx, os.Args[1:], runServer, version, os.Stderr)
}
