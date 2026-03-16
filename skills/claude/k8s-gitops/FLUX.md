# Flux GitOps Operations with RootCause Tools

This reference covers Flux source and reconciliation workflows, multi-environment operations, and troubleshooting using RootCause tools.

## RootCause Tools Used

- `k8s.flux_detect`
- `k8s.diagnose_flux`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.events_timeline`

## Core Flux CRDs

### Source objects

- `GitRepository`
- `OCIRepository`
- `Bucket`
- `HelmRepository`
- `HelmChart`

### Reconciliation objects

- `Kustomization`
- `HelmRelease`

### Image automation objects

- `ImageRepository`
- `ImagePolicy`
- `ImageUpdateAutomation`

## Detection Workflow

1. Run `k8s.flux_detect`.
2. Identify control-plane namespace and controller footprint.
3. Confirm available CRDs for source and reconciliation layers.
4. Capture baseline warnings with `k8s.events`.

## Reconciliation Model

- Source objects fetch and materialize revision artifacts.
- Reconciliation objects apply manifests/charts at intervals.
- Ready conditions signal healthy convergence.
- Degraded status usually indicates source, render, or apply failure.

## Workflow: Source to Apply

1. Source fetch succeeds.
2. Artifact revision is updated.
3. Reconciliation object consumes artifact.
4. Apply/upgrade executes.
5. Health checks pass and Ready condition turns True.

## Common Workflows

### Workflow 1: Cluster-wide Flux status sweep

1. Run `k8s.diagnose_flux`.
2. Identify failing source objects.
3. Identify failing Kustomization and HelmRelease resources.
4. Prioritize by business impact.

### Workflow 2: GitRepository fetch failures

1. Run `k8s.diagnose_flux`.
2. Run `k8s.describe` on failing GitRepository.
3. Inspect auth, URL, branch, and revision errors.
4. Verify corrected source and re-observe timeline.

### Workflow 3: Kustomization not Ready

1. Run `k8s.describe` on Kustomization.
2. Inspect sourceRef, path, dependsOn, and health checks.
3. Use `k8s.events_timeline` to correlate retries and failures.
4. Re-run `k8s.diagnose_flux` after source fix.

### Workflow 4: HelmRelease reconcile failures

1. Run `k8s.describe` on HelmRelease.
2. Verify chart source and values alignment.
3. Inspect retry behavior and failure reasons.
4. Validate state stabilization with timeline and re-diagnosis.

## Suspend and Resume Guidance

Flux objects may be suspended intentionally during maintenance.

### Detection

1. Run `k8s.describe` and inspect `spec.suspend`.
2. Confirm suspension intent in change timeline using `k8s.events_timeline`.

### Resume verification

1. Resume via platform-native process outside this toolset.
2. Run `k8s.diagnose_flux` to verify reconciliation restarts.
3. Ensure Ready condition converges and no warning loops continue.

## Multi-Environment Setup Practices

- Separate source repositories or branches per environment.
- Use environment-specific Kustomization paths.
- Keep promotion flow deterministic and auditable.
- Avoid shared mutable values between prod and non-prod.

### Promotion sequence

1. Validate source environment with `k8s.diagnose_flux`.
2. Promote revision/tag in Git workflow.
3. Validate target environment with `k8s.diagnose_flux`.
4. Confirm healthy convergence with `k8s.events_timeline`.

## Troubleshooting Playbooks

### Playbook A: Flux not detected

1. Run `k8s.flux_detect`.
2. If not detected, verify cluster scope and credentials.
3. Check if GitOps platform is ArgoCD-only.

### Playbook B: Source fetch authentication failures

1. Run `k8s.diagnose_flux`.
2. Run `k8s.describe` on source object.
3. Confirm secrets/credentials references and endpoint validity.
4. Re-check state after credential update.

### Playbook C: Kustomization apply errors

1. Run `k8s.describe` on Kustomization.
2. Inspect build/apply error reasons.
3. Correlate with namespace events using `k8s.events`.
4. Re-run diagnose to confirm fix.

### Playbook D: HelmRelease install/upgrade loops

1. Run `k8s.describe` on HelmRelease.
2. Validate source chart and values mapping.
3. Use `k8s.events_timeline` for retry cadence and hook failures.
4. Confirm stabilization after correction.

## Force Reconciliation Guidance

When reconciliation appears stale:

1. Capture object conditions with `k8s.describe`.
2. Confirm controller availability with `k8s.flux_detect` and `k8s.diagnose_flux`.
3. Trigger platform-native reconcile outside this guide.
4. Validate new events and Ready transition using `k8s.events_timeline`.

## Drift Detection Practices

1. Run `k8s.diagnose_flux` periodically.
2. Use `k8s.list` to inventory Kustomization/HelmRelease objects.
3. Use `k8s.describe` on NotReady objects.
4. Correlate operator actions and drift windows via `k8s.events_timeline`.

## Failure Signature Table

| Signature | Primary object | Primary tool | Follow-up |
|---|---|---|---|
| auth failed | GitRepository/HelmRepository | `k8s.describe` | `k8s.events` |
| path not found | Kustomization | `k8s.describe` | `k8s.diagnose_flux` |
| chart resolve failed | HelmChart/HelmRelease | `k8s.describe` | `k8s.events_timeline` |
| reconcile timeout | Kustomization/HelmRelease | `k8s.diagnose_flux` | `k8s.describe` |
| suspended object | any Flux CR | `k8s.describe` | `k8s.events_timeline` |

## Operational Guardrails

- Keep reconciliation intervals appropriate for environment criticality.
- Avoid bulk source and values changes in one commit.
- Use staged promotion with clear rollback references.
- Track repeated reconcile failures as reliability debt.

## Reporting Template

Include:

1. Flux detection status and namespace.
2. Failing source and reconcile objects with reasons.
3. Condition transition timeline and likely trigger.
4. Ordered remediations.
5. Verification evidence post-remediation.

## Quick Triage Checklist

- `k8s.flux_detect` confirms platform.
- `k8s.diagnose_flux` identifies top failures.
- `k8s.describe` captures object-level conditions.
- `k8s.events` and `k8s.events_timeline` explain chronology.

## Frequent Root Causes

- Invalid source credentials or endpoint.
- Broken source path or branch.
- Missing dependency in Kustomization graph.
- Helm values or chart version mismatch.
- Object suspended unintentionally.

## Verification Exit Criteria

- Source objects report healthy/ready.
- Kustomization and HelmRelease converge to Ready.
- No repeated warning events in recent timeline.
- Promotion pipeline remains stable across environments.

## Related Docs

- `SKILL.md` for dual-platform GitOps triage.
- `ARGOCD.md` for ArgoCD state and troubleshooting model.
