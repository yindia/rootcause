# ArgoCD GitOps Operations with RootCause Tools

This reference focuses on ArgoCD workflows, resource states, and troubleshooting using RootCause MCP tools.

## RootCause Tools Used

- `k8s.argocd_detect`
- `k8s.diagnose_argocd`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## Core ArgoCD CRDs

### Application

- Represents desired app state from Git/Helm source.
- Tracks sync status, health, and operation history.
- Primary object for most incident triage.

### ApplicationSet

- Generates multiple Applications from templates and generators.
- Drift or misconfiguration can fan out quickly.
- Requires template and generator validation during incidents.

## Detection Workflow

1. Run `k8s.argocd_detect`.
2. Identify ArgoCD control-plane namespace(s).
3. Confirm resources discovered include Application/ApplicationSet.
4. Capture baseline warnings using `k8s.events`.

## State Model

### Sync states

- `Synced`: desired and live state aligned.
- `OutOfSync`: drift exists between desired and live.
- `Unknown`: controller cannot determine sync state.

### Health states

- `Healthy`: workload health checks pass.
- `Progressing`: changes in progress.
- `Degraded`: app unhealthy.
- `Missing`: expected resources absent.
- `Unknown`: health cannot be assessed.

## State Interpretation Matrix

| Sync | Health | Meaning | Immediate action |
|---|---|---|---|
| Synced | Healthy | desired state achieved | monitor |
| Synced | Degraded | infra/app issue despite sync | inspect workload events |
| OutOfSync | Healthy | drift likely manual or source lag | identify drift source |
| OutOfSync | Degraded | high-risk mismatch and failure | prioritize fix urgently |
| Unknown | Unknown | controller visibility issue | inspect control-plane health |
| Synced | Progressing | rollout underway | verify convergence |

## Common Workflows

### Workflow 1: Daily health sweep

1. Run `k8s.diagnose_argocd`.
2. Identify apps not in Synced/Healthy.
3. Use `k8s.list` to inventory impacted app objects.
4. Use `k8s.describe` on top offenders.

### Workflow 2: App stuck OutOfSync

1. Run `k8s.diagnose_argocd`.
2. Run `k8s.describe` on the Application.
3. Inspect sync policy and destination details.
4. Correlate with `k8s.events_timeline`.

### Workflow 3: App Degraded after sync

1. Confirm state via `k8s.diagnose_argocd`.
2. Run `k8s.describe` on Application.
3. Run `k8s.events` in app namespace.
4. Validate rollout and dependency readiness.

### Workflow 4: ApplicationSet fan-out failure

1. Use `k8s.list` for ApplicationSet resources.
2. Run `k8s.describe` on the failing ApplicationSet.
3. List generated Applications via `k8s.list`.
4. Identify template drift affecting many apps.

## Troubleshooting Playbooks

### Playbook A: ArgoCD not detected

1. Run `k8s.argocd_detect`.
2. If not detected, confirm cluster selection and access scope.
3. Validate if GitOps stack is Flux-only.

### Playbook B: Repeated sync failures

1. Run `k8s.diagnose_argocd`.
2. Use `k8s.describe` for operation history clues.
3. Use `k8s.events_timeline` to confirm periodic failure cadence.
4. Fix source path/revision/values mismatch and recheck.

### Playbook C: Healthy controller, unhealthy app

1. Run `k8s.diagnose_argocd`.
2. Run `k8s.describe` for app health details.
3. Run `k8s.events` in destination namespace for runtime failures.
4. Treat as workload issue, not controller issue.

### Playbook D: Drift after manual hotfix

1. Confirm OutOfSync via `k8s.diagnose_argocd`.
2. Capture timeline using `k8s.events_timeline`.
3. Reconcile by updating Git source to desired target state.
4. Verify Synced/Healthy restoration.

## Force Reconciliation Guidance

When stale state persists:

1. Capture current status with `k8s.describe`.
2. Confirm no control-plane warnings with `k8s.events`.
3. Trigger platform-native refresh/reconciliation outside this toolset.
4. Re-run `k8s.diagnose_argocd` and confirm state transition.

## Environment Promotion Practices

- Use deterministic source revision flow across environments.
- Promote only after lower environment is Synced/Healthy.
- Use event timeline to detect hidden reconciliation lag.

### Promotion validation sequence

1. `k8s.diagnose_argocd` on source environment.
2. Promote revision in Git.
3. `k8s.diagnose_argocd` on target environment.
4. `k8s.events_timeline` for rollout side effects.

## Drift Detection Practices

1. Baseline state with `k8s.diagnose_argocd`.
2. Enumerate apps with `k8s.list`.
3. Investigate drifted apps using `k8s.describe`.
4. Correlate change windows with `k8s.events_timeline`.

## Operational Guardrails

- Avoid broad auto-sync in unstable environments without safeguards.
- Require review for ApplicationSet template changes.
- Track recurring app failures and classify source vs runtime causes.

## Reporting Template

Include in every report:

1. ArgoCD detection status and namespace.
2. Count of apps in each sync/health state.
3. Top failing apps and condition reasons.
4. Timeline-aligned root cause hypothesis.
5. Ordered remediation and verification evidence.

## Quick Triage Checklist

- `k8s.argocd_detect` confirms installation.
- `k8s.diagnose_argocd` identifies failing apps.
- `k8s.describe` captures object-level reasons.
- `k8s.events` and `k8s.events_timeline` explain failure chronology.

## Frequent Root Causes

- Invalid source path or revision.
- Destination namespace/permissions mismatch.
- Runtime dependency failure despite successful sync.
- ApplicationSet template/generator drift.
- Manual changes causing persistent drift.

## Verification Exit Criteria

- Target apps are Synced and Healthy.
- No repeating warning events.
- No unresolved operation failures in recent timeline.
- Promotion path remains stable across environments.

## Related Docs

- `SKILL.md` for dual-platform GitOps workflow.
- `FLUX.md` for Flux-specific source/reconcile behavior.
