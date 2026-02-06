# RootCause 🧭

[![Go](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-stdio-4A90E2)](https://modelcontextprotocol.io/)

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
- [Tools and Features](#tools-and-features)
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

Optional toolchains return “not detected” when the control plane is absent. Additional toolchains can be registered via the plugin SDK; see `PLUGINS.md`.

---

## Tools and Features

### ✅ Core Kubernetes (`k8s.*` + kubectl-style aliases)
- **CRUD + discovery**: `k8s.get`, `k8s.list`, `k8s.describe`, `k8s.create`, `k8s.apply`, `k8s.patch`, `k8s.delete`, `k8s.api_resources`, `k8s.crds`
- **Ops + observability**: `k8s.logs`, `k8s.events`, `k8s.context`, `k8s.explain_resource`, `k8s.ping`
- **Workload operations**: `k8s.scale`, `k8s.rollout`
- **Execution and access**: `k8s.exec`, `k8s.exec_readonly` (allowlisted), `k8s.port_forward`
- **Debugging**: `k8s.overview`, `k8s.crashloop_debug`, `k8s.scheduling_debug`, `k8s.hpa_debug`, `k8s.vpa_debug`, `k8s.network_debug`, `k8s.private_link_debug`
- **Maintenance**: `k8s.cleanup_pods`, `k8s.node_management`
- **Graph and topology**: `k8s.graph` (Ingress/Service/Endpoints/Workloads + mesh + NetworkPolicy)
- **Metrics**: `k8s.resource_usage` (metrics-server)

### 🕸️ Linkerd (`linkerd.*`)
- `linkerd.health`, `linkerd.proxy_status`, `linkerd.identity_issues`, `linkerd.policy_debug`, `linkerd.cr_status`
- `linkerd.virtualservice_status`, `linkerd.destinationrule_status`, `linkerd.gateway_status`, `linkerd.httproute_status`

### 🌐 Istio (`istio.*`)
- `istio.health`, `istio.proxy_status`, `istio.config_summary`
- `istio.service_mesh_hosts`, `istio.discover_namespaces`, `istio.pods_by_service`, `istio.external_dependency_check`
- `istio.proxy_clusters`, `istio.proxy_listeners`, `istio.proxy_routes`, `istio.proxy_endpoints`, `istio.proxy_bootstrap`, `istio.proxy_config_dump`
- `istio.cr_status`, `istio.virtualservice_status`, `istio.destinationrule_status`, `istio.gateway_status`, `istio.httproute_status`

### 🚀 Karpenter (`karpenter.*`)
- `karpenter.status`, `karpenter.node_provisioning_debug`

### ⎈ Helm (`helm.*`)
- `helm.repo_add`, `helm.repo_list`, `helm.repo_update`
- `helm.list`, `helm.status`
- `helm.install`, `helm.upgrade`, `helm.uninstall`
- `helm.template_apply`, `helm.template_uninstall`

### Kubectl-style aliases
The `k8s.*` tools also expose aliases like `kubectl_get`, `kubectl_list`, `kubectl_describe`, `kubectl_create`, `kubectl_apply`, `kubectl_delete`, `kubectl_logs`, `kubectl_patch`, `kubectl_scale`, `kubectl_rollout`, `kubectl_context`, `kubectl_generic`, `kubectl_top`, `explain_resource`, `list_api_resources`, and `ping`.

---

## Safety Modes

- `--read-only`: removes apply/patch/delete/exec tools from discovery.
- `--disable-destructive`: removes delete and risky write tools unless allowlisted (create/scale/rollout remain available).

---

## Config and Flags

```
rootcause --config config.example.toml --toolsets k8s,linkerd,istio,karpenter,helm
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
- Toolsets live under `toolsets/` and register namespaced tools (`k8s.*`, `linkerd.*`, `karpenter.*`) through a shared MCP registry.

The MCP server runs over stdio using the MCP Go SDK and is designed for local kubeconfig usage. Optional in-cluster deployment is intentionally out of scope for Phase 1.

### Config Reload

Send SIGHUP to reload config and rebuild the tool registry.
On Windows, SIGHUP is not supported; restart the process to reload config.

---

## MCP Transport

RootCause uses MCP over stdio by default (required). HTTP/SSE is not implemented in Phase 1.

---

## Future Cloud Readiness

The toolset system is designed to add cloud integrations (AWS/GCP/Azure) later without changing the core MCP or shared Kubernetes libraries.

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
