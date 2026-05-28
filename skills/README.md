# RootCause Agent Skills

Reusable agent skills for Kubernetes and RootCause workflows. Each skill gives your AI assistant structured guidance: triggers, priority rules, tool quick reference, workflow steps, troubleshooting, and output contracts.

Skills are defined by a schema-versioned manifest embedded in the CLI: `internal/skills/catalog/manifest.json`.

## Quick Install

```bash
# Copy all skills to Claude
cp -r skills/claude/* ~/.claude/skills/

# Or install specific skills
cp -r skills/claude/k8s-helm ~/.claude/skills/
```

## Sync into Project Agent Directories

```bash
# List supported targets
rootcause sync-skills --list-agents

# Sync using project-local default directory for one agent
rootcause sync-skills --agent opencode --project-dir .

# UX helpers
rootcause sync-skills --all-agents --dry-run
rootcause sync-skills --agent claude --skill k8s-incident --skill rootcause-rca
rootcause sync-skills --list-skills
```

## User Custom Skills

Create the default cross-platform config and custom skill folder first:

```bash
rootcause init-config
```

This writes `${HOME}/.rootcause/config.yaml` on macOS/Linux and `%USERPROFILE%\.rootcause\config.yaml` on Windows, with all built-in toolsets enabled and `~/.rootcause/skills` configured.

Users can add their own skills in any folder that follows the same directory shape:

```text
~/.rootcause/skills/
  team-runbook/
    SKILL.md
  oncall-handoff/
    SKILL.md
```

Each custom `SKILL.md` can include standard YAML front matter. `tags` control which MCP tool calls receive that skill as guidance:

```markdown
---
category: Root Cause Analysis
description: Team-specific RCA checklist
tags: [rootcause, rca, payments]
---
# Team RCA

Always check the payments dashboard before declaring database root cause.
```

Tag matching is automatic for every tool call:

- toolset tags: `rootcause`, `k8s`, `helm`, `aws`, etc.
- exact tool tags: `rootcause.rca_generate`, `k8s.events_timeline`
- tool-name tokens: `rca`, `generate`, `events`, `timeline`
- caller-provided tags: pass `skillTags` or `customSkillTags` as a string or string array in tool arguments

Only tagged custom skills are injected into tool call results as `customSkillGuidance`; untagged skills remain available for sync and `skill://...` resources.

For all RootCause incident and issue analysis, tag the skill with `rootcause`. That applies it to every `rootcause.*` tool, including `rootcause.incident_bundle`, `rootcause.rca_generate`, `rootcause.remediation_playbook`, `rootcause.postmortem_export`, and `rootcause.change_timeline`.

Use narrower tags when the guidance should only apply to part of the flow:

| Goal | Recommended tags |
|---|---|
| All RootCause issue workflows | `[rootcause]` |
| RCA drafting only | `[rca]` or `[rootcause.rca_generate]` |
| Kubernetes issue analysis plus RootCause workflows | `[rootcause, k8s, incident]` |
| A team/service-specific workflow | `[rootcause, payments]` plus pass `skillTags: ["payments"]` when needed |

Sync custom skills into agent skill folders with:

```bash
rootcause sync-skills --agent opencode --include-custom
rootcause sync-skills --agent claude --custom-dir ~/.rootcause/skills --skill team-runbook
```

Custom skill names must not collide with built-in skills unless `--custom-overrides` is set.

To expose custom skills to MCP clients, configure the server:

```toml
[skills]
custom_dirs = ["~/.rootcause/skills", "./skills/custom"]
allow_custom_overrides = false
```

MCP clients can then read:

- `skill://catalog` for the merged built-in/custom skill list
- `skill://team-runbook` for a custom skill's `SKILL.md` content

All tool calls include matching configured custom skills in their response metadata/payload as `customSkillGuidance`, allowing MCP agents to evaluate incidents with team-specific instructions and runbooks.

Claude, Codex, OpenCode, and any MCP-compatible agent can use configured custom skills through RootCause MCP responses and `skill://...` resources without syncing skills into that agent's native skill directory. `rootcause sync-skills` is optional and only needed when you also want native agent skill discovery outside MCP tool calls.

Do not put secrets, credentials, kubeconfigs, tokens, or private incident data in custom `SKILL.md` files. Matching skills can be returned in MCP tool responses for the connected client to read.

| Agent | Format | Project Directory |
|---|---|---|
| Claude Code | `SKILL.md` | `.claude/skills/` |
| Cursor | `.mdc` | `.cursor/skills/` |
| Codex | `SKILL.md` | `.codex/skills/` |
| Gemini CLI | `SKILL.md` | `.gemini/skills/` |
| OpenCode | `SKILL.md` | `.opencode/skills/` |
| GitHub Copilot | `Markdown` | `.github/skills/` |
| Windsurf | `Markdown` | `.windsurf/skills/` |
| Devin | `Markdown` | `.devin/skills/` |
| Aider | `SKILL.md` | `.aider/skills/` |
| Sourcegraph Cody | `SKILL.md` | `.cody/skills/` |
| Amazon Q | `SKILL.md` | `.amazonq/skills/` |

## Layout

```
skills/claude/<skill-name>/SKILL.md
```

## All Skills (21 total)

### Incident Response

| Skill | Description |
|---|---|
| `k8s-incident` | Active incident triage: connectivity check, blast radius, evidence collection, ranked hypotheses |
| `rootcause-rca` | RCA workflow: incident bundle, RCA generation, remediation playbook, postmortem export |

### Core & Operations

| Skill | Description |
|---|---|
| `k8s-core` | Core Kubernetes CRUD, logs, events, context switching, and resource discovery |
| `k8s-operations` | Safe operations: restart safety checks, best-practice scoring, mutation preflight |

### Diagnostics & Debugging

| Skill | Description |
|---|---|
| `k8s-diagnostics` | Deep single-workload debugging: crashloop, scheduling, config, HPA, VPA |
| `k8s-troubleshoot` | Keyword-driven and graph-driven debug flows for unknown failure modes |

### Deployment & Delivery

| Skill | Description |
|---|---|
| `k8s-deploy` | Deployment workflows: rollout, scale, restart with preflight safety gates |
| `k8s-helm` | Helm chart lifecycle: search, install, upgrade, diff, rollback advisor |
| `k8s-rollouts` | Progressive delivery: canary, blue-green, and rollout status tracking |

### GitOps

| Skill | Description |
|---|---|
| `k8s-gitops` | ArgoCD and Flux: detect installation, diagnose sync/reconciliation health, drift detection |

### Networking

| Skill | Description |
|---|---|
| `k8s-networking` | Service networking, NetworkPolicy analysis, ingress, and private link debugging |
| `k8s-service-mesh` | Istio and Linkerd: proxy status, config, routing, and mesh health |
| `k8s-cilium` | Cilium endpoint, policy, and node health diagnostics |

### Security & Policy

| Skill | Description |
|---|---|
| `k8s-security` | RBAC, ServiceAccount permissions, IRSA, and security audit workflows |
| `k8s-policy` | Kyverno policy readiness, report failures, and admission control diagnostics |
| `k8s-gatekeeper` | Gatekeeper constraint/template diagnostics and admission-denial analysis |
| `k8s-certs` | cert-manager certificate and issuer readiness, renewal failures |

### Cost & Scaling

| Skill | Description |
|---|---|
| `k8s-cost` | Resource usage analysis, workload right-sizing, and cost optimization recommendations |
| `k8s-autoscaling` | HPA, VPA, and Karpenter node provisioning diagnostics |

### Storage

| Skill | Description |
|---|---|
| `k8s-storage` | PVC binding, PV matching, VolumeAttachment errors, and storage class analysis |

### Browser Automation

| Skill | Description |
|---|---|
| `k8s-browser` | Browser automation for Grafana screenshots, ingress health checks, and dashboard exports |

## Install (Claude)

```bash
cp -r skills/claude/* ~/.claude/skills/
```

## Install a single skill

```bash
cp -r skills/claude/k8s-incident ~/.claude/skills/
```

## SkillKit Note

If you manage skills via the SkillKit ecosystem, use `skills add` with agent targets.
For local project repositories, `rootcause sync-skills` is the recommended path.

```bash
# Example SkillKit install from a remote skill repo
npx skills add owner/repo -a claude-code -a codex
```


Supported agents include Claude, Cursor, Codex, Gemini CLI, GitHub Copilot, Goose, Windsurf, Roo, Amp, and more.

## Auto-generated Catalog

Generate and verify skills docs:

```bash
go run ./cmd/cataloggen
```

Output file: `skills/CATALOG.md`

Tip: keep this file and `README.md` in sync whenever skills are added or renamed.
