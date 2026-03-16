# Skill: k8s-rollouts

Safe rollout and rollback execution framework with explicit preflight gates,
restart safety checks, and Helm rollback advisory evidence.

## Trigger Phrases

Use this skill when the user mentions:
- rollout stuck
- restart deployment safely
- rollback release
- canary failed
- blue green cutover issue
- pods not updating
- new version unhealthy
- helm rollback target

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.rollout` (actions: `status` or `restart`, with `confirm=true` for restart)
- `k8s.restart_safety_check`
- `k8s.best_practice`
- `k8s.safe_mutation_preflight`
- `k8s.describe`
- `k8s.events`
- `helm.rollback_advisor`
- `helm.diff_release`
- `helm.status`

## Rollout Safety Model

Never mutate first.

Required order before restart:
1. `k8s.restart_safety_check`
2. `k8s.best_practice`
3. `k8s.safe_mutation_preflight`
4. `k8s.rollout` restart with `confirm=true`

If any safety gate is red, document and remediate before restart.

## Deployment Strategy Comparison

| Strategy | Strength | Risk | Best Use Case |
|---|---|---|---|
| Rolling update | simple, built-in | partial outage if probes weak | most stateless services |
| Canary | controlled exposure | more routing complexity | high-risk feature changes |
| Blue-green | fast rollback switch | duplicate capacity cost | strict availability requirements |
| Recreate | deterministic replacement | downtime window | non-critical or maintenance workloads |

This skill supports diagnostics across these strategies, even when actual controller remains Deployment/Helm based.

## Safe Rollout Workflow

### Step 1: Check current rollout health

Use `k8s.rollout` status:
```yaml
action: status
kind: Deployment
name: payments-api
namespace: payments
confirm: true
```

Then gather warning evidence:
```yaml
namespace: payments
involvedObjectKind: Deployment
involvedObjectName: payments-api
```
Tool: `k8s.events`.

### Step 2: Run restart safety check

Use `k8s.restart_safety_check`:
```yaml
namespace: payments
name: payments-api
minReadyReplicas: 2
maxUnavailableRatio: 0.25
```

Review:
- ready replica floor,
- PDB compatibility,
- current rollout pressure.

### Step 3: Evaluate workload best practices

Use `k8s.best_practice`:
```yaml
kind: Deployment
name: payments-api
namespace: payments
```

Critical findings to resolve before restart:
- no readiness/liveness probes,
- missing requests/limits,
- single-replica critical workload,
- weak disruption posture.

### Step 4: Mutation preflight

Use `k8s.safe_mutation_preflight`:
```yaml
operation: rollout
kind: Deployment
name: payments-api
namespace: payments
```

Only proceed when preflight passes.

### Step 5: Execute restart

Use `k8s.rollout` restart:
```yaml
action: restart
kind: Deployment
name: payments-api
namespace: payments
confirm: true
```

### Step 6: Verify post-restart behavior

1. rerun `k8s.rollout` status,
2. inspect `k8s.events` for new warnings,
3. use `k8s.describe` if pods remain unhealthy.

## Helm Rollback Advisory Workflow

When workload is Helm-managed:

### Step H1: Current release state

Use `helm.status`:
```yaml
namespace: payments
release: payments
```

### Step H2: Get safer rollback candidates

Use `helm.rollback_advisor`:
```yaml
namespace: payments
release: payments
historyLimit: 20
```

Interpret output:
- avoid blind rollback to immediately previous revision,
- prefer revision with stable success markers.

### Step H3: Preview change impact

Use `helm.diff_release` against target chart/version values.

Example:
```yaml
namespace: payments
release: payments
chart: payments-chart
version: 1.8.2
includeCRDs: false
```

### Step H4: Decide rollback path

If diff indicates expected safe revert:
- proceed with release operation via appropriate Helm workflow.

If diff shows risky collateral:
- choose alternate revision or patch-forward path.

## Restart Safety Pattern

Use this reusable pattern for critical services:
1. ensure at least N ready replicas,
2. ensure PDB supports at least one disruption,
3. ensure probes are healthy,
4. run preflight,
5. restart during controlled window,
6. monitor status and events immediately.

## Common Rollout Failure Modes

| Failure Symptom | Likely Root Cause | Confirm With | Recovery Direction |
|---|---|---|---|
| rollout stuck `0/N` available | failing image pull or startup probe | `k8s.events`, `k8s.describe` | fix image/probe, then continue rollout |
| rollout progressing too slowly | readiness never passing | `k8s.describe` pod conditions | correct readiness logic or dependencies |
| restart increases errors | unsafe single replica restart | `k8s.restart_safety_check` | scale out before restart |
| rollback candidate still risky | hidden config drift | `helm.diff_release`, `helm.status` | pick different revision or targeted patch |
| repeated crash after rollback | data/schema incompatibility | events + workload description | run compatibility recovery plan |

## Parameter Guidance

### `k8s.rollout`

- `action`: `status` for read path, `restart` for mutation path.
- `confirm`: must be `true` for restart execution.

### `k8s.restart_safety_check`

- tune `minReadyReplicas` and `maxUnavailableRatio` based on service criticality.

### `k8s.safe_mutation_preflight`

- always specify exact operation and object identity.

### `helm.rollback_advisor`

- increase history depth for long-lived releases.

### `helm.diff_release`

- use same values context expected during rollback decision.

## Example YAML

### Deployment with robust rolling update

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: payments
spec:
  replicas: 4
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    spec:
      containers:
        - name: api
          image: ghcr.io/acme/payments-api:2.4.1
          readinessProbe:
            httpGet:
              path: /readyz
              port: 8080
```

### PodDisruptionBudget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: payments-api-pdb
  namespace: payments
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: payments-api
```

## Output Contract

Always report:
1. safety gate outcomes,
2. rollout health evidence,
3. recommended action (restart, rollback, or hold),
4. rollback candidate rationale (if Helm-managed),
5. post-action verification checklist.

Example:
```text
Safety checks: pass (min ready replicas satisfied, PDB allows disruption, preflight clean).
Rollout status: stalled due readiness probe failures on new ReplicaSet.
Decision: hold restart, fix readiness endpoint path mismatch first.
If rollback needed: helm.rollback_advisor recommends revision 42; helm.diff_release shows only image and env rollback.
Verify: rollout status healthy and no new warning events for 10 minutes.
```

## Completion Criteria

Rollout operation is complete when:
- safety gates were executed and documented,
- chosen action is evidence-backed,
- rollback reasoning is explicit for Helm releases,
- verification confirms stable post-change state.
