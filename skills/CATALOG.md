# Skills Catalog

Auto-generated from `internal/skills/catalog/manifest.json`.
Do not edit manually; run `go run ./cmd/cataloggen`.

Schema: `1.0.0` | Catalog version: `2026.03.16` | Total skills: `21`

## Browser Automation

| Skill | Description | Source Path |
|---|---|---|
| `k8s-browser` | Browser-driven ingress checks, screenshots, and dashboard exports. | `skills/claude/k8s-browser/SKILL.md` |

## Core and Operations

| Skill | Description | Source Path |
|---|---|---|
| `k8s-core` | Core Kubernetes CRUD, logs, events, and discovery workflows. | `skills/claude/k8s-core/SKILL.md` |
| `k8s-operations` | Safe operations: restart checks, best-practice scoring, mutation preflight. | `skills/claude/k8s-operations/SKILL.md` |

## Cost and Scaling

| Skill | Description | Source Path |
|---|---|---|
| `k8s-autoscaling` | HPA, VPA, and Karpenter provisioning diagnostics. | `skills/claude/k8s-autoscaling/SKILL.md` |
| `k8s-cost` | Resource usage analysis and right-sizing recommendations. | `skills/claude/k8s-cost/SKILL.md` |

## Deployment and Delivery

| Skill | Description | Source Path |
|---|---|---|
| `k8s-deploy` | Deployment workflows for rollout, scale, and restart with safety gates. | `skills/claude/k8s-deploy/SKILL.md` |
| `k8s-helm` | Helm lifecycle: search, install, diff, upgrade, rollback advisor. | `skills/claude/k8s-helm/SKILL.md` |
| `k8s-rollouts` | Progressive delivery patterns: canary, blue-green, rollout verification. | `skills/claude/k8s-rollouts/SKILL.md` |

## Diagnostics and Debugging

| Skill | Description | Source Path |
|---|---|---|
| `k8s-diagnostics` | Deep workload debugging for crashloop, scheduling, config, HPA, VPA. | `skills/claude/k8s-diagnostics/SKILL.md` |
| `k8s-troubleshoot` | Keyword-driven and graph-driven debug flows for unknown failures. | `skills/claude/k8s-troubleshoot/SKILL.md` |

## GitOps

| Skill | Description | Source Path |
|---|---|---|
| `k8s-gitops` | ArgoCD and Flux detect/diagnose workflows for drift and reconciliation. | `skills/claude/k8s-gitops/SKILL.md` |

## Incident Response

| Skill | Description | Source Path |
|---|---|---|
| `k8s-incident` | Active incident triage with blast-radius and evidence-first analysis. | `skills/claude/k8s-incident/SKILL.md` |
| `rootcause-rca` | RCA workflow with timeline, remediation playbook, and postmortem export. | `skills/claude/rootcause-rca/SKILL.md` |

## Networking and Mesh

| Skill | Description | Source Path |
|---|---|---|
| `k8s-cilium` | Cilium endpoint, policy, and node health diagnostics. | `skills/claude/k8s-cilium/SKILL.md` |
| `k8s-networking` | Service networking, NetworkPolicy analysis, ingress and private-link debugging. | `skills/claude/k8s-networking/SKILL.md` |
| `k8s-service-mesh` | Istio and Linkerd proxy, routing, and policy diagnostics. | `skills/claude/k8s-service-mesh/SKILL.md` |

## Security and Policy

| Skill | Description | Source Path |
|---|---|---|
| `k8s-certs` | cert-manager issuer and certificate renewal diagnostics. | `skills/claude/k8s-certs/SKILL.md` |
| `k8s-gatekeeper` | Gatekeeper constraint/template health and admission denial diagnostics. | `skills/claude/k8s-gatekeeper/SKILL.md` |
| `k8s-policy` | Kyverno policy readiness, report failures, and admission diagnostics. | `skills/claude/k8s-policy/SKILL.md` |
| `k8s-security` | RBAC, ServiceAccount, IRSA, and security audit workflows. | `skills/claude/k8s-security/SKILL.md` |

## Storage

| Skill | Description | Source Path |
|---|---|---|
| `k8s-storage` | PVC/PV/VolumeAttachment and storage class troubleshooting. | `skills/claude/k8s-storage/SKILL.md` |

