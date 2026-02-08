# RootCause 🧭

[![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-stdio-4A90E2)](https://modelcontextprotocol.io/)
[![codecov](https://codecov.io/gh/yindia/rootcause/graph/badge.svg?token=F85C1M50K6)](https://codecov.io/gh/yindia/rootcause)

RootCause is a local-first MCP server that helps operators manage Kubernetes resources and identify the real root cause of failures through interoperable toolsets.

Built in Go for a fast, single-binary workflow that competes with npx-based MCP servers while staying kubeconfig-native. ⚡

**Mission statement**: “RootCause is a local-first MCP server that helps operators manage Kubernetes resources and identify the real root cause of failures through interoperable toolsets.”

Inspired by:
- https://github.com/containers/kubernetes-mcp-server
- https://github.com/Flux159/mcp-server-kubernetes

---

## Contents

- [Why RootCause](#why-rootcause)
- [Quick Start](#quick-start-)
- [Installation](#installation)
- [Usage](#usage)
- [MCP Client Example (stdio)](#mcp-client-example-stdio)
- [MCP Client Setup](#mcp-client-setup)
- [Toolchains](#toolchains)
- [Tools](#tools)
- [Safety Modes](#safety-modes)
- [Config and Flags](#config-and-flags)
- [Kubeconfig Resolution](#kubeconfig-resolution)
- [Architecture Overview](#architecture-overview)
- [MCP Transport](#mcp-transport)
- [Future Cloud Readiness](#future-cloud-readiness)
- [Collaboration](#collaboration-)
- [Development](#development)

---

## Why RootCause

- **Local-first**: Uses your kubeconfig identity only. No API keys required.
- **Interoperable toolchains**: K8s, Linkerd, Istio, and Karpenter share the same clients, evidence, and render logic.
- **Fast and portable**: One static Go binary, stdio-first MCP transport.
- **Competitive by design**: Go binary speed and distribution with parity vs npx-based MCP servers.
- **Debugging built-in**: Structured reasoning with likely root causes, evidence, and next checks.
- **Plugin-ready**: Clean SDK to add toolchains without duplicating K8s logic.
- **⭐ Like it?** Star the repo to help us grow and keep shipping.

## Quick Start 🚀

1) Run the server:

```
go run ./cmd/rootcause --config config.example.toml
```

2) Use your existing kubeconfig (default) or point to one:

- Uses `KUBECONFIG` if set, otherwise `~/.kube/config`.
- Override with `--kubeconfig` and `--context`.

3) Connect your MCP client using stdio.

RootCause is built for local development. No API keys are required in this version.

---

## Installation

Homebrew:

```
brew install yindia/homebrew-yindia/rootcause
```

Curl install:

```
curl -fsSL https://raw.githubusercontent.com/yindia/rootcause/refs/heads/main/install.sh | sh
```

Go install:

```
go install ./cmd/rootcause
```

Or build a local binary:

```
go build -o rootcause ./cmd/rootcause
```

Supported OS: macOS, Linux, and Windows.

Windows build example:

```
go build -o rootcause.exe ./cmd/rootcause
```

---

## Usage

Run with a config file:

```
rootcause --config config.toml
```

Enable a subset of toolchains:

```
rootcause --toolsets k8s,istio
```

Enable read-only mode:

```
rootcause --read-only
```

---

## MCP Client Example (stdio)

```
rootcause --config config.toml
```

Point your MCP client to run the command above and use stdio transport.

---

## MCP Client Setup

All MCP clients need the same three fields:
- `command`: the RootCause binary
- `args`: CLI flags (`--config`, `--toolsets`, etc.)
- `env`: optional environment variables like `KUBECONFIG`

### Codex CLI

Add an MCP server entry pointing to RootCause (format varies by client version). Example:

```
[mcp.servers.rootcause]
command = "rootcause"
args = ["--config", "/path/to/config.toml"]
env = { KUBECONFIG = "/path/to/kubeconfig" }
```

### Claude Desktop

Add RootCause to the MCP servers section (use your local config path). Example:

```
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/path/to/config.toml"],
      "env": { "KUBECONFIG": "/path/to/kubeconfig" }
    }
  }
}
```

### GitHub Copilot (VS Code)

If your Copilot/VS Code build supports MCP servers, add a server entry with the RootCause command. Example:

```
"mcp.servers": {
  "rootcause": {
    "command": "rootcause",
    "args": ["--config", "/path/to/config.toml"],
    "env": { "KUBECONFIG": "/path/to/kubeconfig" }
  }
}
```

If the MCP settings key differs in your client, map the fields above to its configuration format.

---

## Toolchains

Enabled by default:
- `k8s`
- `linkerd`
- `karpenter`
- `istio`
- `helm`
- `aws`

Optional toolchains return “not detected” when the control plane is absent. Additional toolchains can be registered via the plugin SDK; see `PLUGINS.md`.

---

## Tools

See `TOOLS.md` for the full tool catalog, quick picker, and graph-first debugging flow references.

---

## Safety Modes

- `--read-only`: removes apply/patch/delete/exec tools from discovery.
- `--disable-destructive`: removes delete and risky write tools unless allowlisted (create/scale/rollout remain available).

---

## Config and Flags

```
rootcause --config config.example.toml --toolsets k8s,linkerd,istio,karpenter,helm,aws
```

### Flags

- `--kubeconfig`
- `--context`
- `--toolsets` (comma-separated)
- `--config`
- `--read-only`
- `--disable-destructive`
- `--log-level`

If `--config` is not set, RootCause will use the `ROOTCAUSE_CONFIG` environment variable when present.

---

## AWS Credentials

The AWS IAM tools use the standard AWS credential chain and region resolution. Set `AWS_REGION` or `AWS_DEFAULT_REGION` (defaults to `us-east-1`), optionally select a profile with `AWS_PROFILE` or `AWS_DEFAULT_PROFILE`, and use any of the normal credential sources (env vars, shared config/credentials files, SSO, or instance metadata).

---

## Kubeconfig Resolution

If `--kubeconfig` is not set, RootCause follows standard Kubernetes loading rules: it uses `KUBECONFIG` when present, otherwise defaults to `~/.kube/config`.

Authentication and authorization use your kubeconfig identity only in this version.

---

## Architecture Overview

RootCause is organized around shared Kubernetes plumbing and toolsets that reuse it.

- Shared clients (typed, dynamic, discovery, RESTMapper) are created once in `internal/kube` and injected into all toolsets.
- Common safeguards live in `internal/policy` (namespace vs cluster enforcement and tool allowlists) and `internal/redact` (token/secret redaction).
- `internal/evidence` gathers events, owner chains, endpoints, and pod status summaries used by all toolsets.
- `internal/render` enforces a consistent analysis output format (root causes, evidence, next checks, resources examined) and provides the shared describe helper.
- Toolsets live under `toolsets/` and register namespaced tools (`k8s.*`, `linkerd.*`, `karpenter.*`, `istio.*`, `helm.*`, `aws.iam.*`, `aws.vpc.*`) through a shared MCP registry.

The MCP server runs over stdio using the MCP Go SDK and is designed for local kubeconfig usage. Optional in-cluster deployment is intentionally out of scope for Phase 1.

### Config Reload

Send SIGHUP to reload config and rebuild the tool registry.
On Windows, SIGHUP is not supported; restart the process to reload config.

---

## MCP Transport

RootCause uses MCP over stdio by default (required). HTTP/SSE is not implemented in Phase 1.

---

## Future Cloud Readiness

AWS IAM support is now available. The toolset system is designed to add deeper cloud integrations (EKS/EC2/VPC/GCP/Azure) without changing the core MCP or shared Kubernetes libraries.

---

## Collaboration 🤝

We welcome collaborators, reviewers, and plugin authors. If you want to add toolsets, improve heuristics, or build cloud integrations, open an issue or PR. Help us make RootCause the fastest, most interoperable Kubernetes MCP server in the ecosystem.

---

## Development

- Config example: `config.toml`
- Plugin SDK guide: `PLUGINS.md`

Run unit tests:

```
go test ./...
```
