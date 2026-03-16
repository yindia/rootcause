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
