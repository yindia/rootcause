# Plugin SDK

RootCause exposes a small SDK so toolsets (toolchains/plugins) can be built in-repo or in a separate module and still share the same tool registry and evidence/rendering helpers.

## Quick Start (External Toolchain)

1. Create a new module:

```bash
mkdir rootcause-aws
cd rootcause-aws
go mod init github.com/acme/rootcause-aws
go get github.com/your-org/rootcause@latest
```

2. Add your toolset under `toolsets/aws`.
3. Register it in `init()` so RootCause can discover it.
4. Build a custom binary that blank-imports your toolset.

## Create a Toolset

1. Create a Go package that implements the toolset interface.
2. Register the toolset in `init()` so it is discovered at runtime.
3. Use shared services or the tool invoker if you need cross-tool interactions.

```go
package aws

import (
	"rootcause/pkg/sdk"
)

type Toolset struct {
	ctx sdk.ToolsetContext
}

func New() *Toolset { return &Toolset{} }

func (t *Toolset) ID() string      { return "aws" }
func (t *Toolset) Version() string { return "0.1.0" }

func (t *Toolset) Init(ctx sdk.ToolsetContext) error {
	if ctx.Clients == nil {
		return fmt.Errorf("missing kube clients")
	}
	// Example: register a shared client for other toolsets.
	_ = ctx.Services.Register("aws.session", mySession)
	return nil
}

func (t *Toolset) Register(reg sdk.Registry) error {
	return reg.Add(sdk.ToolSpec{
		Name:        "aws.status",
		Description: "Example tool",
		ToolsetID:   t.ID(),
		InputSchema: map[string]any{"type": "object"},
		Safety:      sdk.SafetyReadOnly,
		Handler:     t.handleStatus,
	})
}

func init() {
	sdk.MustRegisterToolset("aws", New)
}
```

## Cross-Tool Interaction

- Shared services: `ctx.Services.Register("key", svc)` and `ctx.Services.Get("key")`.
- Internal tool calls: `ctx.CallTool(ctx, req.User, "k8s.describe", args)` (uses policy + audit).

This keeps policy checks and audit logging consistent across toolsets.

## Build a Custom Binary

Create a small `main` in your own module, import RootCause’s server runner and your toolsets, and run it:

```go
package main

import (
	"context"
	"os"

	"rootcause/pkg/server"

	_ "rootcause/toolsets/k8s"
	_ "rootcause/toolsets/linkerd"
	_ "github.com/acme/rootcause-aws/toolsets/aws"
)

func main() {
	if err := server.Run(context.Background(), server.Options{Version: "0.1.0", Stderr: os.Stderr}); err != nil {
		panic(err)
	}
}
```

Toolsets are enabled via `--toolsets` or config like usual.

## Enable Your Toolchain

Example `config.toml`:

```toml
toolsets = ["k8s", "linkerd", "aws"]
```

Or CLI:

```
--toolsets k8s,linkerd,aws
```

## Notes

- Toolchains are compiled in; dynamic Go plugins are not used (keeps a single static binary).
- Use `ctx.Services` for shared clients or caches so other toolsets can reuse your logic.
