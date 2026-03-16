# k8s-helm Troubleshooting

This guide is for diagnosing and recovering Helm release issues with RootCause MCP tools.

Scope:
- installation failures,
- upgrade failures,
- rollback problems,
- chart development and values issues,
- release status interpretation.

Use RootCause tool names only:
- `helm.repo_add`, `helm.repo_list`, `helm.repo_update`
- `helm.list_charts`, `helm.get_chart`, `helm.search_charts`
- `helm.list`, `helm.status`, `helm.diff_release`, `helm.rollback_advisor`
- `helm.install`, `helm.upgrade`, `helm.uninstall`
- `helm.template_apply`, `helm.template_uninstall`
- `k8s.describe`, `k8s.list`, `k8s.events`, `k8s.events_timeline`, `k8s.get`

## Fast Triage Sequence

When an operation fails, use this order:

1. `helm.status`
2. `k8s.events_timeline`
3. `k8s.describe` for failing workloads
4. `helm.diff_release`
5. `helm.rollback_advisor` (if post-upgrade regression)

Do not retry mutation blindly.

## Installation Failures

### 1) Error: cannot re-use a name that is still in use

Cause:
- release name already exists in namespace,
- release was partially removed,
- stale state from previous failed install.

Checks:
1. Run `helm.list` in target namespace.
2. Run `helm.status` for conflicting release name.
3. Run `k8s.list` to inspect remaining release-owned objects.

Recovery options:
- If release is active and intended: run `helm.upgrade` instead of `helm.install`.
- If release is broken and must be recreated: run `helm.uninstall` with `confirm=true`, then reinstall.
- If cleanup is partial: inspect blockers via `k8s.describe` and events.

Prevention:
- enforce unique release naming convention,
- check `helm.list` before install,
- automate namespace+release uniqueness.

### 2) Error: install timed out waiting for condition

Cause:
- pods not ready,
- image pull failures,
- probe misconfiguration,
- PVC binding delays,
- quota or scheduling constraints.

Checks:
1. `helm.status` for notes and last known state.
2. `k8s.events` for warning messages.
3. `k8s.describe` on failing Deployment/StatefulSet/Pod.
4. `k8s.list` for workload and PVC status.

Recovery:
1. Fix root readiness issue (image, probe, resources, storage).
2. Preview with `helm.diff_release`.
3. Re-run `helm.upgrade` with corrected values.

Prevention:
- validate readiness and liveness probes,
- validate image tags and pull secrets,
- right-size resource requests.

### 3) Error: resource already exists

Cause:
- chart attempts to create object already present,
- conflicting ownership annotations,
- previous non-Helm resource in same namespace.

Checks:
1. `helm.status` and `helm.diff_release`.
2. `k8s.get` for conflicting object.
3. `k8s.describe` to inspect owner references and labels.

Recovery:
- align release naming and resource naming templates,
- if existing object should be managed elsewhere, refactor chart values,
- if object belongs to stale release, clean old release safely.

Prevention:
- avoid hardcoded global names in templates,
- include release-scoped naming patterns.

## Upgrade Failures

### 1) Error: another operation is in progress

Cause:
- concurrent install/upgrade/rollback/uninstall attempt,
- release left in pending state.

Checks:
1. `helm.status` for pending state.
2. `helm.list` for release-wide state view.
3. `k8s.events_timeline` for recent lock-triggering actions.

Recovery:
1. Stop parallel operations.
2. Validate safe target with `helm.rollback_advisor` if needed.
3. Compare with `helm.diff_release`.
4. Continue with a single controlled `helm.upgrade`.

Prevention:
- serialize release operations per namespace+release,
- avoid CI jobs that mutate the same release simultaneously.

### 2) Error: immutable field / immutable selector

Cause:
- Deployment or StatefulSet selector changed,
- immutable Service or PVC field changed.

Checks:
1. `helm.diff_release` to identify immutable field drift.
2. `k8s.get` current live resource.
3. `k8s.describe` for immutable-field error events.

Recovery:
- redesign migration path:
  - staged rollout with new resource name,
  - traffic cutover strategy,
  - planned old resource removal.

Do not force blind retries.

Prevention:
- keep selectors stable across chart versions,
- treat selector/name changes as migration events.

## Rollback Issues

### Problem: rollback candidate is unclear

Approach:
1. Run `helm.rollback_advisor` to rank safer targets.
2. Run `helm.diff_release` against intended target state.
3. Prefer revision with known healthy outcomes and low config drift.

### Problem: rollback does not restore health

Possible reasons:
- external dependencies changed,
- rollback target includes hidden breaking config,
- persistent data schema drift.

Checks:
1. `helm.status` after rollback attempt.
2. `k8s.events` and `k8s.describe` on critical workloads.
3. `k8s.get` on Services/Ingress/PVC for dependency mismatches.

Next actions:
- choose alternate advised target,
- apply corrective values with controlled `helm.upgrade`,
- verify with events timeline.

### Problem: release stuck in pending state after rollback

Checks:
1. `helm.status` for pending-* state details.
2. `k8s.events_timeline` for failed hooks/jobs.
3. `k8s.list` for resources not converging.

Recovery:
- resolve hook failures,
- rerun one controlled operation,
- avoid launching competing operations.

## Chart Development Issues

### 1) Template rendering problems

Symptoms:
- malformed YAML,
- missing keys,
- wrong template include behavior,
- rendered resources not matching expectation.

Checks:
1. Use `helm.diff_release` to inspect rendered target behavior.
2. Compare rendered intent against live state from `k8s.get`.
3. Validate template assumptions in chart helpers and values.

Recovery:
- fix templates and helper usage,
- rerun `helm.diff_release`,
- apply via `helm.template_apply` with `confirm=true`.

### 2) Values not applied as expected

Symptoms:
- changes in values file do not appear in workloads,
- old config remains in live resources,
- environment override not effective.

Checks:
1. Confirm target chart version using `helm.get_chart`.
2. Confirm intended release via `helm.status`.
3. Use `helm.diff_release` with explicit values input.
4. Validate live spec with `k8s.get`.

Common causes:
- wrong values file precedence,
- typo in values key path,
- template references wrong key,
- upgrade pointed at unexpected chart version.

Recovery:
- correct values key path and precedence,
- rerun diff,
- execute `helm.upgrade` with `confirm=true`.

## Release Status Reference Table

| Status | Meaning | Typical Action |
|---|---|---|
| `deployed` | Last revision applied successfully | Verify workload health and SLO impact |
| `failed` | Last operation failed | Run triage sequence and inspect events |
| `pending-install` | Install in progress or blocked | Check hooks, events, and convergence |
| `pending-upgrade` | Upgrade in progress or blocked | Stop concurrent ops, inspect pending causes |
| `pending-rollback` | Rollback in progress or blocked | Inspect hook jobs and advised rollback target |
| `uninstalling` | Release removal in progress | Inspect finalizers and hook cleanup |
| `uninstalled` | Release removed from active set | Validate resource cleanup outcome |
| `superseded` | Older revision replaced by newer one | Reference for history and rollback context |
| `unknown` | State not fully resolved | Gather `helm.status` + k8s evidence immediately |

## Debug Command Reference (RootCause Tools)

Repository and discovery:
- `helm.repo_list`
- `helm.repo_add`
- `helm.repo_update`
- `helm.search_charts`
- `helm.list_charts`
- `helm.get_chart`

Release diagnostics:
- `helm.list`
- `helm.status`
- `helm.diff_release`
- `helm.rollback_advisor`

Mutation and recovery:
- `helm.install` (`confirm=true`)
- `helm.upgrade` (`confirm=true`)
- `helm.uninstall` (`confirm=true`)
- `helm.template_apply` (`confirm=true`)
- `helm.template_uninstall` (`confirm=true`)

Kubernetes evidence gathering:
- `k8s.list`
- `k8s.get`
- `k8s.describe`
- `k8s.events`
- `k8s.events_timeline`

## Minimal Recovery Playbooks

Install failed:
1. `helm.status`
2. `k8s.events`
3. `k8s.describe`
4. `helm.diff_release`
5. `helm.upgrade` with corrected values

Upgrade failed:
1. `helm.status`
2. `helm.diff_release`
3. `k8s.events_timeline`
4. `helm.rollback_advisor`
5. controlled `helm.upgrade` recovery

Uninstall blocked:
1. `helm.status`
2. `k8s.list`
3. `k8s.describe`
4. resolve finalizer/hook blockers
5. re-run controlled uninstall path

## Related Documents

- Skill workflow and policy: `skills/claude/k8s-helm/SKILL.md`
- Chart authoring and layout: `skills/claude/k8s-helm/references/CHART-STRUCTURE.md`
