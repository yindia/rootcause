# Skill: k8s-gitops

GitOps diagnostics and operational guidance for ArgoCD and Flux using RootCause MCP tool names only.

## Scope

- Detect whether ArgoCD and/or Flux are installed.
- Diagnose sync and reconciliation health.
- Analyze drift and rollout failures.
- Correlate change timelines to incidents.
- Guide forced reconciliation workflows safely.
- Support environment promotion paths with verification.

## Tooling Rule

- Use RootCause tool names only.
- Prefer `k8s.argocd_detect` and `k8s.flux_detect` before deep diagnosis.
- Use `k8s.diagnose_argocd` and `k8s.diagnose_flux` for first-pass triage.
- Use `k8s.describe`, `k8s.list`, `k8s.events`, and `k8s.events_timeline` for evidence detail.

## Canonical Tool Set

- `k8s.argocd_detect`
- `k8s.diagnose_argocd`
- `k8s.flux_detect`
- `k8s.diagnose_flux`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## Triggers

- gitops
- argocd
- flux
- out of sync
- app degraded
- reconciliation failed
- kustomization not ready
- helmrelease failed
- source fetch failed
- drift detected
- promotion blocked
- environment mismatch
- sync loop

## Priority Rules

| Condition | First action | Why |
|---|---|---|
| Unknown platform | Run `k8s.argocd_detect` and `k8s.flux_detect` | Fast platform fingerprint |
| ArgoCD suspected issue | Run `k8s.diagnose_argocd` | Consolidates sync + health + events |
| Flux suspected issue | Run `k8s.diagnose_flux` | Consolidates source + reconcile signals |
| Need object-level reason | Run `k8s.describe` | Full status/conditions/events |
| Need inventory | Run `k8s.list` | Lists all relevant CRs |
| Need chronology | Run `k8s.events_timeline` | Aligns failure to changes |

## ArgoCD Application States

| Sync status | Health status | Interpretation | Action |
|---|---|---|---|
| Synced | Healthy | Desired and live match | Monitor only |
| Synced | Degraded | Live matches desired but app unhealthy | Inspect workload failures |
| OutOfSync | Healthy | Drift exists but app currently serving | Investigate drift source |
| OutOfSync | Degraded | Drift plus unhealthy app | Prioritize immediate remediation |
| Unknown | Unknown | Controller cannot evaluate | Check controller availability and access |
| Synced | Progressing | Deployment in-flight | Validate rollout convergence |

## Flux Resource Types to Track

| Type | Role in pipeline | Typical failure signal |
|---|---|---|
| GitRepository | Source fetch and revision tracking | auth/branch/revision fetch errors |
| OCIRepository | OCI source pull | tag digest resolution failures |
| Bucket | Object storage source | credentials or endpoint failures |
| Kustomization | Apply and health reconciliation | build/apply/health timeout |
| HelmRepository | Helm source index | repo access and index errors |
| HelmChart | Chart packaging fetch | chart version resolution issues |
| HelmRelease | Helm install/upgrade reconciliation | failed release, hooks, values mismatch |
| ImageRepository | image metadata scanning | registry auth or API failures |
| ImagePolicy | release selection policy | no matching tags |
| ImageUpdateAutomation | git patch commit workflow | git push/branch conflicts |

## Investigation Workflow

### Phase 1: Platform detection and scope

1. Run `k8s.argocd_detect`.
2. Run `k8s.flux_detect`.
3. Determine control-plane namespaces and available CRDs.
4. Capture namespace-level warning events with `k8s.events`.

### Phase 2: First-pass diagnosis

1. If ArgoCD detected, run `k8s.diagnose_argocd`.
2. If Flux detected, run `k8s.diagnose_flux`.
3. Capture top failing resources, reasons, and timestamps.

### Phase 3: Object-level deep dive

1. Use `k8s.list` to enumerate failing Application/ApplicationSet or Kustomization/HelmRelease resources.
2. Use `k8s.describe` for each failing object.
3. Record condition transitions, reason fields, and event messages.

### Phase 4: Timeline and drift correlation

1. Run `k8s.events_timeline` for control-plane namespace.
2. Run `k8s.events_timeline` for impacted app namespace.
3. Align failures with deployment or config changes.

### Phase 5: Remediation and verification

1. Apply source or manifest fixes through GitOps repo workflow.
2. Observe reconciliation through `k8s.diagnose_argocd` or `k8s.diagnose_flux`.
3. Validate condition transitions using `k8s.describe`.
4. Confirm no new warnings in `k8s.events`.

## ArgoCD Path

### Detection

1. Run `k8s.argocd_detect`.
2. Identify namespace hosting ArgoCD controllers.

### Diagnosis

1. Run `k8s.diagnose_argocd`.
2. Identify applications with OutOfSync/Degraded states.
3. Use `k8s.list` to gather all Application objects.

### Deep inspection

1. Run `k8s.describe` on failing Application.
2. Check destination cluster/namespace settings.
3. Check sync policy and retry behavior.

### Drift handling

1. Confirm drift exists in `k8s.diagnose_argocd` outputs.
2. Identify whether drift source is manual change or source revision mismatch.
3. Validate post-fix state returns to Synced and Healthy.

## Flux Path

### Detection

1. Run `k8s.flux_detect`.
2. Identify controller namespace and available source/reconcile CRDs.

### Diagnosis

1. Run `k8s.diagnose_flux`.
2. Identify failing source objects and reconcile objects.
3. Use `k8s.list` for GitRepository/Kustomization/HelmRelease inventory.

### Deep inspection

1. Run `k8s.describe` on failing source object first.
2. Run `k8s.describe` on failing Kustomization/HelmRelease.
3. Extract failing conditions and reconcile intervals.

### Reconciliation handling

1. Fix source, path, chart, or values issue in Git.
2. Observe fresh reconciliation via `k8s.events_timeline`.
3. Re-run `k8s.diagnose_flux` until Ready conditions stabilize.

## Force Reconciliation Guidance

Use this sequence when a stale state persists:

1. Confirm current failing object with `k8s.describe`.
2. Confirm no control-plane outage with `k8s.argocd_detect` or `k8s.flux_detect` outputs.
3. Trigger reconciliation through platform-native workflow outside this skill.
4. Immediately verify with `k8s.events_timeline` and diagnose tool rerun.
5. Capture before/after condition transition evidence.

## Environment Promotion Guidance

### Promotion model

- Promote from dev -> stage -> prod via versioned Git references.
- Maintain environment-specific overlays/values with explicit ownership.

### Validation sequence per environment

1. Run platform detect tool for environment cluster.
2. Run diagnose tool and ensure current state healthy before promotion.
3. Apply promotion commit through Git flow.
4. Monitor with diagnose + events timeline.
5. Block further promotion on unresolved warnings.

## Drift Detection Strategy

1. Baseline with `k8s.diagnose_argocd` or `k8s.diagnose_flux`.
2. Enumerate impacted resources using `k8s.list`.
3. Inspect object detail with `k8s.describe`.
4. Correlate changes with `k8s.events_timeline`.
5. Close loop by confirming healthy synced state.

## Incident Patterns

| Pattern | Primary signal | First tool | Follow-up |
|---|---|---|---|
| Argo app stuck OutOfSync | sync state mismatch | `k8s.diagnose_argocd` | `k8s.describe` |
| Argo app Degraded after sync | workload health failing | `k8s.describe` | `k8s.events_timeline` |
| Flux source fetch failing | source object not ready | `k8s.diagnose_flux` | `k8s.describe` |
| Flux HelmRelease looping | release reconcile errors | `k8s.describe` | `k8s.events` |
| No GitOps events visible | wrong namespace scope | `k8s.argocd_detect`/`k8s.flux_detect` | `k8s.events_timeline` |

## Troubleshooting Checklists

### ArgoCD checklist

- ArgoCD detected by `k8s.argocd_detect`.
- Failing apps identified by `k8s.diagnose_argocd`.
- Application details reviewed via `k8s.describe`.
- Warning event timeline captured.
- Post-fix state returns to Synced/Healthy.

### Flux checklist

- Flux detected by `k8s.flux_detect`.
- Source and reconcile errors identified via `k8s.diagnose_flux`.
- Object details reviewed via `k8s.describe`.
- Warning event timeline captured.
- Post-fix Ready state stabilized.

## Output Contract

Always return:

- Platform detection result (ArgoCD, Flux, both, or none).
- Failing resources with state/condition reasons.
- Timeline-aligned root causes.
- Ordered remediation actions.
- Verification evidence after each remediation step.

## References

- `ARGOCD.md` for Application/ApplicationSet state model, workflows, and troubleshooting.
- `FLUX.md` for source/reconcile object model, suspend/resume operations, and multi-env practices.

## Related Skills

- `k8s-helm` for Helm-specific release debugging in Flux contexts.
- `k8s-incident` for cross-domain outage triage.
