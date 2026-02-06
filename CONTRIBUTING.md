# Contributing

## Development Setup

- Go 1.23+
- kubeconfig available locally for integration tests
- metrics-server recommended for `k8s.resource_usage`

Run tests:

```
go test ./...
```

## Adding a Toolset

1. Create a new folder under `toolsets/<toolset-id>`.
2. Implement the toolset interface:

```go
ID() string
Version() string
Init(ctx ToolsetContext) error
Register(reg Registry) error
```

3. Register tools with namespaced names (e.g., `mytoolset.*`).
4. Use shared libraries instead of duplicating logic:
   - `internal/kube` for client creation and RESTMapper usage.
   - `internal/policy` for namespace/cluster enforcement.
   - `internal/evidence` for events, owner chains, endpoints, and pod status.
   - `internal/redact` for redaction.
   - `internal/render` for consistent analysis output (use `render.DescribeAnalysis`).
5. Update the Go input schemas in the toolset `schema.go` files.
6. Add unit tests for safety mode behavior or shared helper usage.
7. If your toolset should participate in graph-first flows, extend `k8s.debug_flow` with new nodes/steps.
8. For external toolsets, register via `pkg/sdk` and document how to build a custom binary (see `PLUGINS.md`).

## Shared Libraries

- `internal/kube`: create typed/dynamic/discovery clients once and share across toolsets.
- `internal/policy`: enforce namespace restrictions and tool allowlists.
- `internal/evidence`: gather common evidence like events and owner references.
- `internal/render`: standardize output (root cause, evidence, next checks).
- `internal/redact`: token and secret redaction.

## Integration Test Plan

Use kind or minikube to validate:

1. `k8s.*` tools against core resources and CRDs.
2. Namespace vs cluster enforcement.
3. Safety modes removing tools from discovery.
4. Linkerd toolset (if installed) detects control-plane health.
5. Karpenter toolset (if installed) diagnoses pending pods.
6. `k8s.debug_flow` walks graph edges for traffic/pending/crashloop scenarios.
