# RootCause 🧭

[![Go](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![MCP](https://img.shields.io/badge/MCP-stdio-4A90E2)](https://modelcontextprotocol.io/)
[![codecov](https://codecov.io/gh/yindia/rootcause/graph/badge.svg?token=F85C1M50K6)](https://codecov.io/gh/yindia/rootcause)


**AI-native SRE for Kubernetes incidents.**

RootCause is a local-first MCP server that turns natural-language requests into evidence-backed incident analysis, Kubernetes diagnostics, and safer operations.

Built in Go as a single binary, RootCause is optimized for low-friction local workflows using your existing kubeconfig identity.

---

[🚀 Quick Start](#quick-start-) | [🌐 Client Setup](#mcp-client-setup-) | [🛠️ Tools](#tools) | [🧩 Skills](#agent-skills) | [🔒 Safety](#safety-modes) | [⚙️ Config](#config-and-flags) | [🏗️ Architecture](#architecture-overview) | [🤝 Contributing](#contributing-guide-)

---

## Why RootCause 💡

RootCause is built for SRE/operator workflows where speed matters, but unsafe automation is unacceptable.

- **🚀 Stop context-switching**: investigate incidents, rollout risk, Helm/Terraform/AWS signals, and remediation from one MCP server.
- **🧠 AI-powered diagnostics**: evidence-first analysis with RCA, timelines, and action-oriented next checks.
- **💸 Built-in cost optimization**: combine resource usage, workload best-practice checks, Terraform plan analysis, and cloud context for optimization decisions.
- **🔒 Enterprise-ready guardrails**: role/namespace policy enforcement, redaction, read-only mode, destructive tool controls, and mutation preflight.
- **⚡ Zero learning curve**: ask natural-language operational questions and use provided prompt templates for common SRE flows.
- **🌐 Universal compatibility**: works with MCP-compatible clients across Claude, Cursor, Copilot, Codex, and more.
- **🏭 Production-grade workflow**: single Go binary, kubeconfig-native auth, deterministic structured outputs, and broad test coverage.

### Why teams choose it

| Need | RootCause answer |
|---|---|
| "What changed and why did this break?" | `rootcause.incident_bundle`, `rootcause.change_timeline`, `rootcause.rca_generate` |
| "Is it safe to restart or roll out now?" | `k8s.restart_safety_check`, `k8s.best_practice`, `k8s.safe_mutation_preflight` |
| "Is my platform ecosystem healthy?" | `k8s.*_detect` + `k8s.diagnose_*` for ArgoCD/Flux/cert-manager/Kyverno/Gatekeeper/Cilium |
| "Can I standardize SRE responses?" | Prompt templates + structured output from shared render/evidence pipeline |

## What Can You Do?

Ask your AI assistant in natural language:

- "Why did this deployment fail after rollout?"
- "Is this workload safe to restart right now?"
- "Why are ArgoCD apps out of sync?"
- "Is Flux healthy in this cluster?"
- "Why are certs failing to renew?"
- "Before patch/apply, is this mutation safe?"

RootCause keeps its depth-first model: evidence-first diagnosis, root-cause analysis, and remediation flow instead of raw tool sprawl.

Power users can map these prompts to concrete tools in this README (`Complete Feature Set`, `Toolchains`, and `Tools` sections).

## Use Cases

### Incident response
- Build end-to-end incident evidence with `rootcause.incident_bundle`
- Generate probable causes with `rootcause.rca_generate`
- Export timeline and postmortem artifacts for follow-up

### Safe operations before mutation
- Evaluate rollout/restart risk with `k8s.restart_safety_check` and `k8s.best_practice`
- Run `k8s.safe_mutation_preflight` before apply/patch/delete/scale operations

### Ecosystem-specific health checks
- ArgoCD: detect installation and diagnose sync/health drift
- Flux: detect controllers and diagnose reconciliation failures
- cert-manager / Kyverno / Gatekeeper / Cilium: detect footprint and diagnose control-plane or policy issues

## Feature Highlights

| Area | RootCause Capability |
|---|---|
| Incident analysis | `rootcause.incident_bundle`, `rootcause.rca_generate`, `rootcause.change_timeline`, `rootcause.postmortem_export`, `rootcause.capabilities` |
| Kubernetes resilience | `k8s.restart_safety_check`, `k8s.best_practice`, `k8s.safe_mutation_preflight` |
| Ecosystem diagnostics | ArgoCD/Flux/cert-manager/Kyverno/Gatekeeper/Cilium via `*_detect` and `diagnose_*` tools |
| Deployment safety | Automatic preflight before k8s mutating operations |
| Helm operations | Chart search/list/get, release diff, rollback advisor, template apply/uninstall flows |
| Terraform analysis | Module/provider search + `terraform.debug_plan` for impact/risk analysis |
| Service mesh & scaling | Linkerd/Istio/Karpenter diagnostics with shared evidence model |

## Complete Feature Set

| Category | Representative capabilities |
|---|---|
| Kubernetes core (`k8s.*`) | CRUD, logs/events, graph-based debug flows, restart safety, best-practice scoring, mutation preflight |
| Ecosystem diagnostics | ArgoCD, Flux, cert-manager, Kyverno, Gatekeeper, Cilium via `*_detect` and `diagnose_*` |
| Incident intelligence (`rootcause.*`) | Incident bundle orchestration, timeline export, RCA generation, remediation playbook, postmortem export |
| Helm operations (`helm.*`) | Chart registry search/list/get, release status/diff, rollback advisor, install/upgrade/uninstall, template apply/uninstall |
| Terraform analysis (`terraform.*`) | Modules/providers/resources/data source discovery + plan debugging |
| Service mesh (`istio.*`, `linkerd.*`) | Proxy/config/status diagnostics, policy/routing visibility, mesh resource health |
| Cluster autoscaling (`karpenter.*`) | Provisioning, nodepool/nodeclass, interruption and scheduling diagnostics |
| Cloud context (`aws.*`, `gcp.*`) | AWS: IAM, VPC, EC2, EKS, ECR, STS, KMS diagnostics. GCP: Cloud Monitoring metrics + SLOs, Cloud Logging entries, workload-scoped error timelines for cross-layer incident analysis |
| Safety and controls | Read-only mode, destructive gating, explicit confirmation, auto preflight checks before mutating K8s operations |

## Agent Skills

Extend your AI coding agent with Kubernetes and RootCause expertise using the built-in skills library in `skills/`.

Skills metadata is schema-versioned and embedded in the CLI from `internal/skills/catalog/manifest.json`.

### Quick Install

```bash
# Copy all skills to Claude
cp -r skills/claude/* ~/.claude/skills/

# Or install a specific skill
cp -r skills/claude/k8s-helm ~/.claude/skills/
```

### Sync prompts and skills into your agent

One command syncs both prompts (as slash commands) and skills (as agent-side
guidance) into your AI client's native directories. By default everything is
synced; use `--prompts-only` or `--skills-only` for granular control. Custom
prompts under `~/.rootcause/prompts/` and custom skills under
`~/.rootcause/skills/` are picked up automatically.

```bash
# List supported agents (shows both commands + skills directories)
rootcause sync --list-agents

# List everything available (built-in + custom prompts and skills)
rootcause sync --list

# Default: sync both prompts and skills for one agent (project-local)
rootcause sync --agent claude --project-dir .

# User-globally — writes ~/.claude/commands/ and ~/.claude/skills/
rootcause sync --agent claude --user

# All supported agents
rootcause sync --all-agents

# Granular control
rootcause sync --agent claude --prompts-only
rootcause sync --agent claude --skills-only
rootcause sync --agent claude --prompt observability_workload_diagnose
rootcause sync --agent claude --skill k8s-incident

# Existing files are NOT overwritten by default. Opt in:
rootcause sync --agent claude --overwrite

# Dry-run to see what would be written
rootcause sync --agent claude --dry-run

# Ignore custom directories and sync only built-ins
rootcause sync --agent claude --builtin-only
```

Prompts that took `{{namespace}}` / `{{workload}}` tokens become positional
`$1` / `$2` in the generated slash command. Optional `{{name|default}}` tokens
render with a "Defaults" preamble so the agent applies fallbacks when an
argument is omitted.

Per-agent target directories:

| Agent | Slash commands | Skills |
|---|---|---|
| `claude` | `.claude/commands/` | `.claude/skills/` |
| `cursor` | `.cursor/commands/` | `.cursor/skills/` |
| `codex`  | `.codex/commands/`  | `.codex/skills/` |
| `copilot`| `.github/prompts/`  | `.github/skills/` |
| `gemini` | `.gemini/commands/` | `.gemini/skills/` |
| `opencode`| `.opencode/commands/` | `.opencode/skills/` |
| `windsurf`| `.windsurf/commands/` | `.windsurf/skills/` |
| `aider`  | `.aider/commands/`  | `.aider/skills/` |
| `devin`, `cody`, `amazonq` | (no slash-command directory yet) | `.devin/skills/` etc. |

### User Custom Skills

Users can add team or personal skills in a folder containing one subdirectory per skill:

```text
~/.rootcause/skills/
  team-runbook/
    SKILL.md
```

Use YAML front matter to standardize metadata and tags. Tags decide which MCP tool calls receive the skill as guidance:

```markdown
---
category: Root Cause Analysis
description: Team-specific RCA checklist
tags: [rootcause, rca, payments]
---
# Team RCA

Always check the payments dashboard before declaring database root cause.
```

RootCause matches tags on every tool call using the toolset (`rootcause`, `k8s`, `helm`), exact tool name (`rootcause.rca_generate`), tool-name tokens (`rca`, `events`, `timeline`), plus optional call arguments `skillTags` or `customSkillTags`.

For all RootCause incident and issue analysis, tag the skill with `rootcause`. That applies it to every `rootcause.*` tool, including `rootcause.incident_bundle`, `rootcause.rca_generate`, `rootcause.remediation_playbook`, `rootcause.postmortem_export`, and `rootcause.change_timeline`.

Use narrower tags when the guidance should only apply to part of the flow:

| Goal | Recommended tags |
|---|---|
| All RootCause issue workflows | `[rootcause]` |
| RCA drafting only | `[rca]` or `[rootcause.rca_generate]` |
| Kubernetes issue analysis plus RootCause workflows | `[rootcause, k8s, incident]` |
| A team/service-specific workflow | `[rootcause, payments]` plus pass `skillTags: ["payments"]` when needed |

Sync custom skills into supported agent directories:

```bash
rootcause sync --agent opencode --skills-only
rootcause sync --agent claude --custom-skill-dir ~/.rootcause/skills --skill team-runbook
```

Expose custom skills through MCP resources by initializing the home config:

```bash
rootcause init-config
```

or adding them to config manually:

```yaml
skills:
  custom_dirs:
    - "~/.rootcause/skills"
    - "./skills/custom"
  allow_custom_overrides: false
```

MCP clients can read `skill://catalog` for the merged skill list and `skill://team-runbook` for the skill content. Custom names cannot collide with built-ins unless overrides are explicitly enabled.

Every tool call includes matching tagged custom skills in response metadata/payload as `customSkillGuidance`, so MCP agents can consider team-specific runbook instructions during root-cause analysis and other workflows.

Syncing skills into `.claude/`, `.codex/`, `.opencode/`, or other agent-specific directories is optional. Claude, Codex, OpenCode, and any MCP-compatible client can use configured custom skills through RootCause tool responses and `skill://...` resources without local skill sync. Syncing is only needed when you want the agent's native skill system to discover RootCause skills outside MCP tool calls.

Do not put secrets, credentials, kubeconfigs, tokens, or private incident data in custom `SKILL.md` files. Matching skills can be returned in MCP tool responses for the connected client to read.

Skill file formats per agent:

| Agent | Format |
|---|---|
| Claude Code, Codex, Gemini CLI, OpenCode, Aider, Sourcegraph Cody, Amazon Q | `SKILL.md` |
| Cursor | `.mdc` |
| GitHub Copilot, Windsurf, Devin | plain `.md` |

For the matching slash-command directories per agent, see the unified table earlier in this section.

### Available Skills (22)

22 skills are currently included.

| Category | Skills |
|---|---|
| Incident Response | `k8s-incident`, `rootcause-rca` |
| Core and Operations | `k8s-core`, `k8s-operations` |
| Diagnostics and Debugging | `k8s-diagnostics`, `k8s-troubleshoot` |
| Deployment and Delivery | `k8s-deploy`, `k8s-helm`, `k8s-rollouts` |
| GitOps | `k8s-gitops` |
| Networking and Mesh | `k8s-networking`, `k8s-service-mesh`, `k8s-cilium` |
| Security and Policy | `k8s-security`, `k8s-policy`, `k8s-gatekeeper`, `k8s-certs` |
| Cost and Scaling | `k8s-cost`, `k8s-autoscaling` |
| Storage | `k8s-storage` |
| Browser Automation | `k8s-browser` |
| Cloud Observability | `k8s-gcp` |


Supported agents include Claude, Cursor, Codex, Gemini CLI, GitHub Copilot, Goose, Windsurf, Roo, Amp, and more.

Skills include consistent triggers, workflow steps, tool references, troubleshooting notes, and output contracts.

See `skills/README.md` for full documentation and `skills/CATALOG.md` for auto-generated catalog output.

### MCP Resources

Access Kubernetes data as browsable resources:

| Resource URI | Description |
|---|---|
| `kubeconfig://contexts` | List all available kubeconfig contexts |
| `kubeconfig://current-context` | Get current active context |
| `namespace://current` | Get current namespace |
| `namespace://list` | List all namespaces |
| `cluster://info` | Get cluster connection info |
| `cluster://nodes` | Get detailed node information |
| `cluster://version` | Get Kubernetes version |
| `cluster://api-resources` | List available API resources |
| `manifest://deployments/{namespace}/{name}` | Get deployment YAML |
| `manifest://services/{namespace}/{name}` | Get service YAML |
| `manifest://pods/{namespace}/{name}` | Get pod YAML |
| `manifest://configmaps/{namespace}/{name}` | Get ConfigMap YAML |
| `manifest://secrets/{namespace}/{name}` | Get secret YAML (data masked) |
| `manifest://ingresses/{namespace}/{name}` | Get ingress YAML |

### MCP Prompts

Pre-built workflow prompts for Kubernetes and platform operations:

| Prompt | Description |
|---|---|
| `troubleshoot_workload` | Comprehensive troubleshooting guide for pods/deployments |
| `deploy_application` | Step-by-step deployment workflow |
| `security_audit` | Security scanning and RBAC analysis workflow |
| `cost_optimization` | Resource optimization and cost analysis workflow |
| `disaster_recovery` | Backup and recovery planning workflow |
| `debug_networking` | Network debugging for services and connectivity |
| `scale_application` | Scaling guide with HPA/VPA best practices |
| `upgrade_cluster` | Kubernetes cluster upgrade planning |
| `sre_incident_commander` | Severity-based SRE incident coordination workflow |
| `istio_mesh_diagnose` | Diagnose Istio control-plane and traffic policy issues |
| `linkerd_mesh_diagnose` | Diagnose Linkerd control-plane, proxy, and policy health |
| `helm_release_recovery` | Recover failed Helm install/upgrade with rollback strategy |
| `terraform_drift_triage` | Investigate Terraform drift and plan safety |
| `aws_eks_operational_check` | EKS health, nodegroup, and IAM integration diagnostics |
| `karpenter_capacity_debug` | Debug Karpenter provisioning and scheduling issues |
| `observability_workload_diagnose` | Triage a workload via the configured observability backend (works for any cluster) |

### Authoring custom prompts and skills

For a full walkthrough with a realistic example (AWS PrivateLink debugging),
see [`docs/AUTHORING.md`](docs/AUTHORING.md). It covers writing a prompt,
writing a complementary skill, syncing both into your client, and what happens
when the AI runs the resulting workflow end-to-end.

Quick reference below.

#### Adding Custom Prompts (recommended: one file per prompt)

Drop one markdown file per prompt into `~/.rootcause/prompts/`. Each file declares the prompt's metadata in YAML front-matter and the rendered text below.

```
~/.rootcause/prompts/
  team-status.md
  payments-p1-drill.md
  verify-deploy.md
```

Example — `~/.rootcause/prompts/team-status.md`:

```markdown
---
name: team_status
description: Daily status check for a workload
arguments:
  - name: workload
    description: Deployment name
    required: true
  - name: namespace
    description: Namespace (defaults to payments)
    required: false
---

Give me the current health of workload {{workload}} in namespace {{namespace|payments}}.

Check:
- Pod status (running / restarting / pending)
- Recent errors in logs (last 30 minutes)
- Any k8s events that look unusual

Keep it to 5 bullet points.
```

After saving, expose it as a bare `/<name>` slash command:

```bash
rootcause sync --agent claude
```

Resolution order (first match wins for the directory; the legacy single-file path is also loaded and merged on top):

| Priority | Directory (recommended) | Single file (legacy) |
|---|---|---|
| 1 | `ROOTCAUSE_PROMPTS_DIR` env | `MCP_PROMPTS_FILE` env |
| 2 | `[prompts].dir` in `config.yaml` | `ROOTCAUSE_PROMPTS_FILE` env |
| 3 | `~/.rootcause/prompts/` | `[prompts].file` in `config.yaml` |
| 4 | `~/.config/rootcause/prompts/` | `~/.rootcause/prompts.yaml` |
| 5 | `./rootcause-prompts.d/` | `~/.config/rootcause/prompts.yaml`, `./rootcause-prompts.yaml` |

A custom prompt whose `name:` matches a built-in **replaces** it — useful for org-specific overrides of `troubleshoot_workload` etc.

Multi-prompt YAML files are also supported. Place a `*.yaml` file inside the prompts directory with a top-level `prompts:` list:

```yaml
# ~/.rootcause/prompts/security.yaml
prompts:
  - name: security_audit
    description: Org-specific security policy checks
    template: "Run security audit for {{namespace|all namespaces}} with CIS and policy controls."
    arguments:
      - name: namespace
        required: false
```

### Key Capabilities

- 🤖 **Powerful tool catalog** - Kubernetes, ecosystem diagnostics, incident workflows, Helm, Terraform, service mesh, and AWS context.
- 🎯 **Prompt-driven workflows** - Repeatable runbook templates for incident and reliability analysis.
- 📊 **MCP Resources support** - Readable resource URIs for kubeconfig, namespace, cluster, and manifest access.
- 🔐 **Security first** - Non-destructive modes, policy enforcement, secret masking, and mutation preflight checks.
- 🏥 **Advanced diagnostics** - Root-cause oriented outputs with evidence and recommended next actions.
- 🎡 **Strong Helm + Terraform coverage** - Chart lifecycle and plan/debug analysis in one server.
- 🔧 **CLI-first operations** - Single binary, local kubeconfig usage, and toolset-level controls.

## Getting Started

### 1) Run RootCause

```bash
go run . --config config.yaml
```

### 2) Connect your MCP client

Use stdio transport and point your MCP client to the `rootcause` command.

### 3) Try high-signal prompts

- "Generate an incident bundle for namespace payments and summarize the likely root cause."
- "Run best-practice checks for deployment payment-api and list critical findings."
- "Run safe mutation preflight for this apply operation before execution."

## Quick Start 🚀

1) Run the server:

```
go run . --config config.example.yaml
```

2) Use your existing kubeconfig (default) or point to one:

- Uses `KUBECONFIG` if set, otherwise `~/.kube/config`.
- Override with `--kubeconfig` and `--context`.

3) Connect your MCP client using stdio.

RootCause is built for local development. No API keys are required in this version.

> Safe-by-default workflow: diagnose read-only first, then run mutation preflight before any write operation.

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
go install .
```

Or build a local binary:

```
go build -o rootcause .
```

Supported OS: macOS, Linux, and Windows.

Windows build example:

```
go build -o rootcause.exe .
```

### Docker

```bash
# Build local image
docker build -t rootcause:local .

# Run stdio mode (default)
docker run --rm -it rootcause:local
```

RootCause is stdio-only. To expose it to a remote MCP client, run it inside a
process supervisor (or container) and front the stdio pipes with a reverse
proxy (Caddy/Nginx/Tailscale Funnel/SSH).

CI image publishing is configured via GitHub Actions in `.github/workflows/docker.yml` and pushes to GHCR (`ghcr.io/<owner>/rootcause`) on `main` and release tags.

---

## Usage

Initialize a home config with every built-in toolset enabled and custom skills configured:

```bash
rootcause init-config
```

This writes `${HOME}/.rootcause/config.yaml` on macOS/Linux and `%USERPROFILE%\.rootcause\config.yaml` on Windows, creates the sibling `skills/` directory, and refuses to overwrite unless `--overwrite` is passed.

Run with a config file:

```
rootcause --config config.yaml
```

Enable a subset of toolchains:

```
rootcause --toolsets k8s,istio
```

Enable read-only mode:

```
rootcause --read-only
```

Sync prompts (as slash commands) and skills into agent-specific project directories:

```bash
rootcause sync --agent claude --project-dir .
```

See [`docs/AUTHORING.md`](docs/AUTHORING.md) for an end-to-end walkthrough that writes a custom prompt + skill for an AWS PrivateLink debug runbook.

---

## MCP Client Setup 🌐

All MCP clients use the same core values:
- `command`: `rootcause`
- `args`: usually `--config /path/to/config.yaml`
- `env`: optional `KUBECONFIG`

### Universal template

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

## All Supported AI Assistants

### Claude Desktop

File: `~/Library/Application Support/Claude/claude_desktop_config.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### Claude Code

File: `~/.config/claude-code/mcp.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### Cursor

File: `~/.cursor/mcp.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### GitHub Copilot (VS Code)

File: VS Code `settings.json` (MCP-enabled builds)

```json
{
  "mcp.servers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### OpenAI Codex / Codex CLI

Format can vary by release. Equivalent TOML entry:

```toml
[mcp.servers.rootcause]
command = "rootcause"
args = ["--config", "/Users/you/.config/rootcause/config.yaml"]
env = { KUBECONFIG = "/Users/you/.kube/config" }
```

### Goose

File: `~/.config/goose/config.yaml`

```yaml
extensions:
  rootcause:
    command: rootcause
    args:
      - --config
      - /Users/you/.config/rootcause/config.yaml
```

### Gemini CLI

File: `~/.gemini/settings.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### Roo Code / Kilo Code

File: `~/.config/roo-code/mcp.json` or `~/.config/kilo-code/mcp.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### Windsurf

File: `~/.config/windsurf/mcp.json`

```json
{
  "mcpServers": {
    "rootcause": {
      "command": "rootcause",
      "args": ["--config", "/Users/you/.config/rootcause/config.yaml"],
      "env": { "KUBECONFIG": "/Users/you/.kube/config" }
    }
  }
}
```

### Other MCP-compatible clients

Use the universal template and map keys to the client's schema.

## MCP Client Compatibility

Works seamlessly with MCP-compatible AI assistants:

| Client | Status | Client | Status |
|---|---|---|---|
| Claude Desktop | ✅ Native | Claude Code | ✅ Native |
| Cursor | ✅ Native | Windsurf | ✅ Native |
| GitHub Copilot | ✅ Native | OpenAI Codex | ✅ Native |
| Gemini CLI | ✅ Native | Goose | ✅ Native |
| Roo Code | ✅ Native | Kilo Code | ✅ Native |
| Amp | ✅ Compatible | Trae | ✅ Compatible |
| OpenCode | ✅ Compatible | Kiro CLI | ✅ Compatible |
| Antigravity | ✅ Compatible | Clawdbot | ✅ Compatible |
| Droid (Factory) | ✅ Compatible | Any MCP Client | ✅ Compatible |

### Validate setup (all providers)

1. Restart your client after editing MCP config.
2. Ask: "List RootCause tools".
3. Ask: "Run `k8s.argocd_detect`".
4. If tools are missing, verify `rootcause` path, `--toolsets`, and `KUBECONFIG`.

### Suggested first prompts (RootCause context)

- "Run incident bundle for namespace payments and summarize root cause."
- "Check deployment payment-api restart safety before rollout."
- "Diagnose ArgoCD health in namespace argocd."
- "Preflight this patch operation before mutation."

## MCP Client Example (stdio)

```bash
rootcause --config config.yaml
```

Point your MCP client to run the command above and use stdio transport.

---

## Example Operator Flows 🧪

### Incident RCA flow

1. "Create incident bundle for namespace payments"
2. "Generate RCA from latest incident bundle"
3. "Export postmortem draft"

Tools behind this flow:
- `rootcause.incident_bundle`
- `rootcause.rca_generate`
- `rootcause.postmortem_export`

### Safe rollout flow

1. "Run restart safety check for deployment payment-api"
2. "Run best-practice check for payment-api"
3. "Run mutation preflight for rollout restart"

Tools behind this flow:
- `k8s.restart_safety_check`
- `k8s.best_practice`
- `k8s.safe_mutation_preflight`

### Ecosystem diagnosis flow

1. "Detect Flux in this cluster"
2. "Diagnose Flux reconciliation health in namespace flux-system"
3. "Summarize top issues and next actions"

Tools behind this flow:
- `k8s.flux_detect`
- `k8s.diagnose_flux`

---

## Toolchains

Enabled by default:

| Toolchain | Primary Purpose | Typical Requirement |
|---|---|---|
| `k8s` | Core Kubernetes operations and diagnostics | Kubernetes API access |
| `linkerd` | Linkerd health and policy diagnostics | Linkerd control plane |
| `karpenter` | Node provisioning and scaling diagnostics | Karpenter controller |
| `istio` | Service mesh configuration and proxy diagnostics | Istio control plane |
| `helm` | Chart registry/release workflows and diffing | Helm 3 and cluster access |
| `aws` | EKS/EC2/VPC/IAM/ECR/KMS/STS diagnostics | AWS credentials |
| `gcp` | Cloud Monitoring metrics + SLOs, Cloud Logging analysis for any workload shipping telemetry to GCP (GKE, EKS, AKS, or self-managed) | GCP credentials (ADC or `GOOGLE_APPLICATION_CREDENTIALS`); project from `GOOGLE_CLOUD_PROJECT` / `GCP_PROJECT` env, or explicit `projectId` arg |
| `terraform` | Registry and plan impact analysis | Terraform workflows |
| `rootcause` | Incident bundles, RCA, timeline, postmortem export | Kubernetes access |
| `browser` (optional) | Browser automation via agent-browser | `MCP_BROWSER_ENABLED=true` + agent-browser install |

Optional toolchains return "not detected" when the control plane is absent. Additional toolchains can be registered via the plugin SDK; see `PLUGINS.md`.

Enable only what you need:

```bash
rootcause --toolsets k8s,helm,rootcause
```

### Optional: Browser Automation (26 Tools)

Automate web-based Kubernetes operations with [agent-browser](https://github.com/vercel-labs/agent-browser) integration.

Quick setup:

```bash
# Install agent-browser
npm install -g agent-browser
agent-browser install

# Enable browser tools
export MCP_BROWSER_ENABLED=true
rootcause
```

What you can do:

- 🌐 Test deployed apps via Ingress URLs
- 📸 Screenshot Grafana, ArgoCD, or any K8s dashboard
- ☁️ Automate cloud console operations (EKS, GKE, AKS)
- 🏥 Health check web applications
- 📄 Export monitoring dashboards as PDF
- 🔐 Test authentication flows with persistent sessions

26 available tools: `browser_open`, `browser_screenshot`, `browser_click`, `browser_fill`, `browser_test_ingress`, `browser_screenshot_grafana`, `browser_health_check`, and 19 more.

Full list: `browser_open`, `browser_screenshot`, `browser_click`, `browser_fill`, `browser_test_ingress`, `browser_screenshot_grafana`, `browser_health_check`, `browser_snapshot`, `browser_get_text`, `browser_get_html`, `browser_evaluate`, `browser_pdf`, `browser_wait_for`, `browser_wait_for_url`, `browser_press`, `browser_select`, `browser_check`, `browser_uncheck`, `browser_hover`, `browser_type`, `browser_upload`, `browser_drag`, `browser_new_tab`, `browser_switch_tab`, `browser_close_tab`, `browser_close`.

Advanced features:

- Cloud providers: Browserbase, Browser Use
- Persistent browser profiles
- Remote CDP connections
- Session management

---

## Tools

Prompt templates for common debugging flows are in `prompts/prompt.md`.

### Core Kubernetes (`k8s.*` + kubectl-style aliases)

- CRUD + discovery: `k8s.get`, `k8s.list`, `k8s.describe`, `k8s.create`, `k8s.apply`, `k8s.patch`, `k8s.delete`, `k8s.api_resources`, `k8s.crds`
- Ops + observability: `k8s.logs`, `k8s.events`, `k8s.context`, `k8s.explain_resource`, `k8s.ping`, `k8s.events_timeline`
- Workload operations and safety: `k8s.scale`, `k8s.rollout`, `k8s.restart_safety_check`, `k8s.best_practice`, `k8s.safe_mutation_preflight`
- Ecosystem detection: `k8s.argocd_detect`, `k8s.flux_detect`, `k8s.cert_manager_detect`, `k8s.kyverno_detect`, `k8s.gatekeeper_detect`, `k8s.cilium_detect`
- Ecosystem diagnostics: `k8s.diagnose_argocd`, `k8s.diagnose_flux`, `k8s.diagnose_cert_manager`, `k8s.diagnose_kyverno`, `k8s.diagnose_gatekeeper`, `k8s.diagnose_cilium`
- Debugging: `k8s.overview`, `k8s.crashloop_debug`, `k8s.scheduling_debug`, `k8s.hpa_debug`, `k8s.vpa_debug`, `k8s.storage_debug`, `k8s.config_debug`, `k8s.permission_debug`, `k8s.network_debug`, `k8s.private_link_debug`, `k8s.debug_flow`
- Maintenance + topology: `k8s.cleanup_pods`, `k8s.node_management`, `k8s.graph`, `k8s.resource_usage`

### Linkerd (`linkerd.*`)

- `linkerd.health`, `linkerd.proxy_status`, `linkerd.identity_issues`, `linkerd.policy_debug`, `linkerd.cr_status`, `linkerd.virtualservice_status`, `linkerd.destinationrule_status`, `linkerd.gateway_status`, `linkerd.httproute_status`

### Istio (`istio.*`)

- `istio.health`, `istio.proxy_status`, `istio.config_summary`, `istio.service_mesh_hosts`, `istio.discover_namespaces`, `istio.pods_by_service`, `istio.external_dependency_check`
- `istio.proxy_clusters`, `istio.proxy_listeners`, `istio.proxy_routes`, `istio.proxy_endpoints`, `istio.proxy_bootstrap`, `istio.proxy_config_dump`
- `istio.cr_status`, `istio.virtualservice_status`, `istio.destinationrule_status`, `istio.gateway_status`, `istio.httproute_status`

### Karpenter (`karpenter.*`)

- `karpenter.status`, `karpenter.node_provisioning_debug`, `karpenter.nodepool_debug`, `karpenter.nodeclass_debug`, `karpenter.interruption_debug`

### Helm (`helm.*`)

- Repo/registry: `helm.repo_add`, `helm.repo_list`, `helm.repo_update`, `helm.list_charts`, `helm.get_chart`, `helm.search_charts`
- Release operations: `helm.list`, `helm.status`, `helm.diff_release`, `helm.rollback_advisor`, `helm.install`, `helm.upgrade`, `helm.uninstall`, `helm.template_apply`, `helm.template_uninstall`

### AWS IAM (`aws.iam.*`)

- `aws.iam.list_roles`, `aws.iam.get_role`, `aws.iam.get_instance_profile`, `aws.iam.update_role`, `aws.iam.delete_role`
- `aws.iam.list_policies`, `aws.iam.get_policy`, `aws.iam.update_policy`, `aws.iam.delete_policy`

### AWS VPC (`aws.vpc.*`)

- `aws.vpc.list_vpcs`, `aws.vpc.get_vpc`, `aws.vpc.list_subnets`, `aws.vpc.get_subnet`, `aws.vpc.list_route_tables`, `aws.vpc.get_route_table`
- `aws.vpc.list_nat_gateways`, `aws.vpc.get_nat_gateway`, `aws.vpc.list_security_groups`, `aws.vpc.get_security_group`
- `aws.vpc.list_network_acls`, `aws.vpc.get_network_acl`, `aws.vpc.list_internet_gateways`, `aws.vpc.get_internet_gateway`
- `aws.vpc.list_vpc_endpoints`, `aws.vpc.get_vpc_endpoint`, `aws.vpc.list_network_interfaces`, `aws.vpc.get_network_interface`
- `aws.vpc.list_resolver_endpoints`, `aws.vpc.get_resolver_endpoint`, `aws.vpc.list_resolver_rules`, `aws.vpc.get_resolver_rule`

### AWS EC2 (`aws.ec2.*`)

- `aws.ec2.list_instances`, `aws.ec2.get_instance`, `aws.ec2.list_auto_scaling_groups`, `aws.ec2.get_auto_scaling_group`, `aws.ec2.list_load_balancers`, `aws.ec2.get_load_balancer`
- `aws.ec2.list_target_groups`, `aws.ec2.get_target_group`, `aws.ec2.list_listeners`, `aws.ec2.get_listener`, `aws.ec2.get_target_health`
- `aws.ec2.list_listener_rules`, `aws.ec2.get_listener_rule`, `aws.ec2.list_auto_scaling_policies`, `aws.ec2.get_auto_scaling_policy`, `aws.ec2.list_scaling_activities`, `aws.ec2.get_scaling_activity`
- `aws.ec2.list_launch_templates`, `aws.ec2.get_launch_template`, `aws.ec2.list_launch_configurations`, `aws.ec2.get_launch_configuration`
- `aws.ec2.get_instance_iam`, `aws.ec2.get_security_group_rules`, `aws.ec2.list_spot_instance_requests`, `aws.ec2.get_spot_instance_request`
- `aws.ec2.list_capacity_reservations`, `aws.ec2.get_capacity_reservation`, `aws.ec2.list_volumes`, `aws.ec2.get_volume`, `aws.ec2.list_snapshots`, `aws.ec2.get_snapshot`, `aws.ec2.list_volume_attachments`
- `aws.ec2.list_placement_groups`, `aws.ec2.get_placement_group`, `aws.ec2.list_instance_status`, `aws.ec2.get_instance_status`

### AWS EKS (`aws.eks.*`)

- `aws.eks.list_clusters`, `aws.eks.get_cluster`, `aws.eks.list_nodegroups`, `aws.eks.get_nodegroup`, `aws.eks.list_addons`, `aws.eks.get_addon`
- `aws.eks.list_fargate_profiles`, `aws.eks.get_fargate_profile`, `aws.eks.list_identity_provider_configs`, `aws.eks.get_identity_provider_config`
- `aws.eks.list_updates`, `aws.eks.get_update`, `aws.eks.list_nodes`, `aws.eks.debug`

### AWS ECR (`aws.ecr.*`)

- `aws.ecr.list_repositories`, `aws.ecr.describe_repository`, `aws.ecr.list_images`, `aws.ecr.describe_images`, `aws.ecr.describe_registry`, `aws.ecr.get_authorization_token`

### AWS STS (`aws.sts.*`)

- `aws.sts.get_caller_identity`, `aws.sts.assume_role`

### AWS KMS (`aws.kms.*`)

- `aws.kms.list_keys`, `aws.kms.list_aliases`, `aws.kms.describe_key`, `aws.kms.get_key_policy`

### GCP Metrics (`gcp.metrics.*`)

- `gcp.metrics.query` — run a raw Cloud Monitoring MQL query and return time series.
- `gcp.metrics.workload` — CPU, memory, and restart count metrics for a Kubernetes workload over a time window.
- `gcp.metrics.list_descriptors` — list Cloud Monitoring metric descriptors for discoverability (accepts a Monitoring filter).
- `gcp.metrics.slo_list` — enumerate Service Monitoring services and their SLO configuration (goal, period, indicator type). Live burn-rate is out of scope; use `gcp.metrics.query` with MQL `select_slo_burn_rate(...)` when needed.

### GCP Logs (`gcp.logs.*`)

- `gcp.logs.query` — run a raw Cloud Logging filter and return matching entries.
- `gcp.logs.workload` — recent errors/warnings for a Kubernetes workload over a time window.
- `gcp.logs.error_timeline` — bucketed error/warning counts for spotting the inflection point.
- `gcp.logs.correlated_with_bundle` — pull log entries matching a `rootcause.incident_bundle` event window (accepts the bundle object or explicit `startTime`/`endTime`).

### Terraform (`terraform.*`)

- `terraform.debug_plan`
- `terraform.list_modules`, `terraform.get_module`, `terraform.list_module_versions`, `terraform.search_modules`
- `terraform.list_providers`, `terraform.get_provider`, `terraform.list_provider_versions`, `terraform.get_provider_package`, `terraform.search_providers`
- `terraform.list_resources`, `terraform.get_resource`, `terraform.search_resources`
- `terraform.list_data_sources`, `terraform.get_data_source`, `terraform.search_data_sources`

### RootCause (`rootcause.*`)

- `rootcause.incident_bundle`, `rootcause.change_timeline`, `rootcause.rca_generate`, `rootcause.remediation_playbook`, `rootcause.postmortem_export`, `rootcause.capabilities`

`rootcause.incident_bundle` accepts an optional `workload` argument. When provided alongside `namespace`, and the `gcp` toolset is enabled, the default chain automatically appends `gcp.metrics.workload` and `gcp.logs.workload` so the bundle includes GCP-side metrics and logs for that workload. `rca_generate`, `remediation_playbook`, and `postmortem_export` propagate `workload` through to the auto-built bundle as well.

### Browser (`browser_*`, optional)

- `browser_open`, `browser_screenshot`, `browser_click`, `browser_fill`, `browser_test_ingress`, `browser_screenshot_grafana`, `browser_health_check`
- `browser_snapshot`, `browser_get_text`, `browser_get_html`, `browser_evaluate`, `browser_pdf`, `browser_wait_for`, `browser_wait_for_url`
- `browser_press`, `browser_select`, `browser_check`, `browser_uncheck`, `browser_hover`, `browser_type`, `browser_upload`, `browser_drag`
- `browser_new_tab`, `browser_switch_tab`, `browser_close_tab`, `browser_close`

### Kubectl-style aliases

- `kubectl_get`, `kubectl_list`, `kubectl_describe`, `kubectl_create`, `kubectl_apply`, `kubectl_delete`, `kubectl_logs`, `kubectl_patch`, `kubectl_scale`, `kubectl_rollout`, `kubectl_context`, `kubectl_generic`, `kubectl_top`, `explain_resource`, `list_api_resources`, `ping`

---

## Safety Modes

- `--read-only`: removes apply/patch/delete/exec tools from discovery.
- `--disable-destructive`: removes delete and risky write tools unless allowlisted (create/scale/rollout remain available).
- Mutating tools are documented in this README under `Complete Feature Set` and `Safety Modes`.

Default safety policy:
- If a user does not explicitly request a mutating action, treat the request as read-only diagnostics.
- Do not run mutating tools implicitly during analysis.
- For investigation-first workflows, prefer running RootCause in `--read-only` mode.
- K8s mutating tools `create/apply/patch/delete/scale/rollout/cleanup_pods/node_management` run an automatic `k8s.safe_mutation_preflight` check before execution.

Safety workflow recommendation:
1. Run read-only diagnosis (`k8s.*_debug`, `k8s.*_detect`, `k8s.diagnose_*`, `rootcause.incident_bundle`)
2. Run `k8s.safe_mutation_preflight` for intended mutation
3. Execute mutation only after preflight passes and `confirm=true`

---

## Config and Flags

```
rootcause --config config.example.yaml --toolsets k8s,linkerd,istio,karpenter,helm,aws
```

Create a cross-platform home config first if you do not already have one:

```bash
rootcause init-config
```

The generated config enables `k8s`, `linkerd`, `karpenter`, `istio`, `helm`, `aws`, `gcp`, `terraform`, and `rootcause`, sets stdio transport defaults, and configures `~/.rootcause/skills` (skills) and `~/.rootcause/prompts/` (custom prompts) as the user-authored content directories.

### Flags

- `--kubeconfig`
- `--context`
- `--toolsets` (comma-separated)
- `--config`
- `--read-only`
- `--disable-destructive`
- `--log-level`

RootCause speaks **stdio only**. The MCP client spawns the binary and talks to
it over the pipes; there is no HTTP/SSE listener and no in-app auth. For
remote access, put RootCause behind a reverse proxy or SSH and let the proxy
handle TLS + authn.

If `--config` is not set, RootCause will use the `ROOTCAUSE_CONFIG` environment variable when present.

---

## AWS Credentials

The AWS toolset uses the standard AWS credential chain and region resolution. You can supply these via env vars (recommended for ad-hoc use) **or** via the `aws:` section of `config.yaml` (recommended for repeatable setups).

### SSO setup (most common today)

SSO does **not** need `credentials_file`. Run `aws configure sso` to create a profile in `~/.aws/config`, then point RootCause at that profile name:

```yaml
aws:
  region: "us-east-1"
  profile: "platform-sso"
  # credentials_file is intentionally unset for SSO
```

Before the first call (and whenever the SSO session expires) the user runs `aws sso login --profile platform-sso`. The SDK picks up the cached SSO session from `~/.aws/sso/cache/` automatically — no static keys required.

### Static-key setup (legacy / CI)

For static `AKIA*` keys in a non-default shared credentials file:

```yaml
aws:
  region: "us-east-1"
  profile: "platform"
  credentials_file: "~/.aws/credentials.team"
```

### Resolution order (priority high → low)

1. Tool-call argument (per call where applicable)
2. `AWS_REGION` / `AWS_PROFILE` / `AWS_DEFAULT_REGION` / `AWS_DEFAULT_PROFILE` env vars
3. `aws.region` / `aws.profile` / `aws.credentials_file` in `config.yaml`
4. SDK default discovery (shared config, SSO, instance metadata)
5. `us-east-1` region fallback if nothing resolved

`aws.credentials_file` is added to the SDK's shared-credentials path list, so a team-specific credentials file can live alongside the SDK default without touching the env. SSO setups should leave this empty.

---

## Observability Credentials

Observability is a separate config concern from per-cloud auth, because the cloud the workload runs in is often *not* the cloud where its telemetry lives. A common pattern:

- EKS workloads shipping logs and metrics into a centralized GCP project.
- GKE workloads in many projects fanning into one shared "ops" project.
- AWS workloads + Datadog for metrics + GCP Cloud Logging for audit.

Today RootCause ships the GCP backend (Cloud Monitoring + Cloud Logging). Future backends (Datadog, CloudWatch, Grafana/Prometheus) will slot in as siblings.

### GCP backend

```yaml
observability:
  gcp:
    # The GCP project that hosts Cloud Monitoring + Cloud Logging.
    project: "my-ops-project"
    # Optional: override gcp.credentials_file just for observability calls
    # (useful when telemetry lives in a different GCP account than the rest
    # of your estate). Leave empty to reuse gcp.credentials_file.
    # credentials_file: "~/keys/observability-sa.json"
```

### Resolution chains

GCP project (every `gcp.*` observability tool):

1. Explicit `projectId` argument on the tool call.
2. `GOOGLE_CLOUD_PROJECT` env var.
3. `GCP_PROJECT` env var.
4. `observability.gcp.project` from `config.yaml`.

GCP credentials (when targeting observability):

1. `GOOGLE_APPLICATION_CREDENTIALS` env var.
2. `observability.gcp.credentials_file` (per-backend override).
3. `gcp.credentials_file` (cross-cutting GCP auth).
4. Application Default Credentials (`gcloud auth application-default login`).

The observability project is **not** auto-detected from the kubeconfig context — an EKS or AKS cluster can also ship logs and metrics to GCP, so the project must come from the user, not from the cluster's control-plane identity.

When `gcp.metrics.workload` and `gcp.logs.workload` are enabled and `rootcause.incident_bundle` is called with both `namespace` and `workload`, the default bundle chain auto-includes GCP workload metrics and logs alongside the k8s evidence.

## GCP Credentials (non-observability)

The `gcp:` section configures auth for *any* future GCP tool that isn't observability — control-plane operations against the workload's project, IAM, etc.

```yaml
gcp:
  # Service-account key applied to all gcp.* calls unless an observability
  # override is present. Leave empty for ADC (gcloud auth).
  credentials_file: "~/keys/rootcause-sa.json"
```

Today no non-observability GCP tools exist, so in practice this section is consumed only as a credentials fallback for the observability backend.

---

## Kubeconfig Resolution

If `--kubeconfig` is not set, RootCause follows standard Kubernetes loading rules: it uses `KUBECONFIG` when present, otherwise defaults to `~/.kube/config`.

Authentication and authorization use your kubeconfig identity only in this version.

---

## Troubleshooting

### kubeconfig not found
- Verify `KUBECONFIG` or `~/.kube/config`
- Override explicitly with `--kubeconfig /path/to/config`

### tools not visible in MCP client
- Confirm server is running and client points to `rootcause`
- Check selected toolsets with `--toolsets`
- If using `--read-only`, mutating tools will be hidden by design

### ecosystem tools return not detected
- This usually means the ecosystem control plane is not installed in the cluster
- Run `k8s.<ecosystem>_detect` first, then `k8s.diagnose_<ecosystem>`

### mutation blocked by preflight
- Run `k8s.safe_mutation_preflight` explicitly and inspect failed checks
- Fix policy/namespace/resource issues, then retry with `confirm=true`

---

## Architecture at a Glance

```text
AI Client
  -> MCP stdio server
  -> Tool registry (k8s/linkerd/istio/karpenter/helm/aws/terraform/rootcause)
  -> Shared internals (kube clients, evidence, policy, rendering, redaction)
  -> Target APIs (Kubernetes + cloud providers)
```

Why this matters:
- consistent evidence format across toolsets
- reusable diagnostics instead of duplicated logic
- safer operations through centralized policy and preflight checks

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

RootCause speaks **stdio only** — the MCP client (Claude Desktop, Claude
Code, Cline, etc.) launches the binary as a child process and exchanges
JSON-RPC over stdin/stdout. There is no HTTP/SSE listener and no built-in
auth, by design: the trust boundary is the local user.

```bash
rootcause --config config.yaml
```

For remote access, run RootCause on the box that owns the kubeconfig and
front it with a reverse proxy or SSH-tunnel that terminates TLS and handles
authentication. RootCause itself stays a small, local-first tool.

Design focus today:
- best-in-class local reliability for AI-assisted SRE workflows
- deterministic, auditable outputs for incident review
- safe mutation gates instead of broad write-by-default behavior

---

## Future Cloud Readiness

AWS IAM and GCP Cloud Monitoring/Logging support are now available. The toolset system is designed to add deeper cloud integrations (EKS/EC2/VPC/Azure/extended GCP services) without changing the core MCP or shared Kubernetes libraries.

---

## Contributing Guide 🤝

We welcome code, docs, tests, and operational feedback.

### Ways to contribute

- 🐛 Report bugs with reproducible steps and expected behavior
- 💡 Propose features with concrete operator scenarios
- 🧪 Improve tests for safety, policy, and ecosystem diagnostics
- 🧩 Add or improve toolsets via shared SDK and internal libraries

### Contributor workflow

1. Fork and create a feature branch
2. Implement focused changes with tests
3. Run local verification:

```bash
go test ./...
```

4. Update docs (`README.md`, `prompts/prompt.md`) if behavior changed
5. Open PR with problem statement, approach, and verification notes

### Development references

- Contribution rules: `CONTRIBUTING.md`
- Plugin SDK and external toolsets: `PLUGINS.md`
- Config example: `config.yaml`
- MCP eval harness: `eval/README.md`

### PR quality checklist

- [ ] Behavior matches user/operator expectations
- [ ] Safety model preserved (`read-only`, destructive gating, preflight)
- [ ] Tests added/updated for new behavior
- [ ] Tool/docs consistency checked (`README.md`)
