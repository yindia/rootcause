# Skill: k8s-deploy

Safe Kubernetes deployment workflows using RootCause MCP tools only.

This skill enforces one deployment principle:
preflight first, then deploy, then verify evidence.

Never run write operations without safety checks.

## Scope

Use this skill for:
- rollout-based deployment changes on Deployments
- image updates and rollout restarts
- Helm release installation and upgrades
- replica scaling as part of release cutover
- post-deploy health verification and rollback signals

Do not use this skill for:
- deep incident forensics (use `k8s-incident`)
- cluster capacity planning (use `k8s-diagnostics`)
- generalized CRUD exploration (use `k8s-core`)

## Canonical Pattern

Safe deployment pattern:
1. Preflight
2. Deploy
3. Verify
4. Decide continue or roll back

The default path is RollingUpdate for Deployments.
For trade-off guidance, read `references/STRATEGIES.md`.

## Required Tool Order

Before every write path:
1. `k8s.safe_mutation_preflight`
2. Deploy tool (`k8s.apply` or `helm.install` or `helm.upgrade` or `k8s.rollout` or `k8s.scale`)
3. `k8s.rollout` with `action: "status"`
4. `k8s.events_timeline` for warnings
5. `k8s.resource_usage` for saturation signals

## Triggers

Activate when user asks for:
- deploy
- release
- rollout
- restart deployment
- helm install
- helm upgrade
- scale replicas
- canary preparation
- blue-green handoff planning

## Safety Contract

- Every write call must set `confirm: true`
- Preflight must be run before every write call
- Restarts must run `k8s.restart_safety_check` first
- If preflight fails, stop and report blockers
- If rollout status is not healthy, do not chain more writes

## Tool Map

| Stage | RootCause Tool | Why |
|---|---|---|
| Preflight | `k8s.safe_mutation_preflight` | catch policy, quota, disruption, and ownership blockers |
| Restart risk gate | `k8s.restart_safety_check` | avoid unsafe rolling restarts |
| Quality gate | `k8s.best_practice` | identify resilience gaps before release |
| Raw manifest deploy | `k8s.apply` | server-side apply with ownership tracking |
| Rollout monitor | `k8s.rollout` | verify progression to healthy state |
| Scale control | `k8s.scale` | controlled replica changes |
| Release diff | `helm.diff_release` | preview changes before Helm mutation |
| Helm install | `helm.install` | first release creation |
| Helm upgrade | `helm.upgrade` | release update path |
| Helm status | `helm.status` | release health and notes |
| Timeline verification | `k8s.events_timeline` | warning sequence after deploy |
| Runtime pressure | `k8s.resource_usage` | CPU/memory stress after rollout |

## Parameter Examples (RootCause Format)

`k8s.safe_mutation_preflight` for apply:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n  namespace: payments\n..."
}
```

`k8s.safe_mutation_preflight` for scale:
```json
{
  "operation": "scale",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "replicas": 6
}
```

`k8s.safe_mutation_preflight` for rollout restart:
```json
{
  "operation": "rollout",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.restart_safety_check`:
```json
{
  "name": "api",
  "namespace": "payments",
  "minReadyReplicas": 2,
  "maxUnavailableRatio": 0.25
}
```

`k8s.best_practice`:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.apply`:
```json
{
  "confirm": true,
  "namespace": "payments",
  "fieldManager": "rootcause-release",
  "force": false,
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n  namespace: payments\n..."
}
```

`k8s.rollout` status:
```json
{
  "confirm": true,
  "action": "status",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.rollout` restart:
```json
{
  "confirm": true,
  "action": "restart",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.scale`:
```json
{
  "confirm": true,
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "replicas": 6
}
```

`k8s.events_timeline`:
```json
{
  "namespace": "payments",
  "involvedObjectKind": "Deployment",
  "involvedObjectName": "api",
  "includeNormal": false,
  "limit": 60
}
```

`k8s.resource_usage`:
```json
{
  "namespace": "payments",
  "includePods": true,
  "includeNodes": true,
  "sortBy": "cpu",
  "limit": 30
}
```

`helm.diff_release`:
```json
{
  "release": "api",
  "namespace": "payments",
  "chart": "payment-api",
  "repoURL": "https://charts.example.internal",
  "version": "1.12.0",
  "valuesFiles": ["values/prod.yaml"],
  "includeCRDs": true
}
```

`helm.install`:
```json
{
  "confirm": true,
  "release": "api",
  "namespace": "payments",
  "chart": "payment-api",
  "repoURL": "https://charts.example.internal",
  "version": "1.12.0",
  "valuesFiles": ["values/prod.yaml"],
  "createNamespace": true,
  "wait": true,
  "atomic": true,
  "timeoutSeconds": 600
}
```

`helm.upgrade`:
```json
{
  "confirm": true,
  "release": "api",
  "namespace": "payments",
  "chart": "payment-api",
  "repoURL": "https://charts.example.internal",
  "version": "1.13.0",
  "valuesFiles": ["values/prod.yaml"],
  "install": true,
  "wait": true,
  "atomic": true,
  "timeoutSeconds": 600
}
```

`helm.status`:
```json
{
  "release": "api",
  "namespace": "payments"
}
```

## Workflow A: Rolling Update Deployments

1. Run `k8s.safe_mutation_preflight` with `operation: "apply"`.
2. Run `k8s.best_practice` for the target Deployment.
3. Apply Deployment manifest with `k8s.apply`.
4. Track status with `k8s.rollout` action `status`.
5. Validate warnings using `k8s.events_timeline`.
6. Validate resource stability using `k8s.resource_usage`.
7. If unstable, stop further writes and escalate to incident flow.

## Workflow B: Helm Upgrade Release

1. Run `k8s.safe_mutation_preflight` with `operation: "apply"` and rendered manifest if available.
2. Run `helm.diff_release` and review expected changes.
3. Run `helm.upgrade` with `atomic: true` and `wait: true`.
4. Run `helm.status`.
5. Run `k8s.rollout` action `status` for key Deployments.
6. Run `k8s.events_timeline` for release namespace.

## Workflow C: Controlled Rollout Restart

1. Run `k8s.safe_mutation_preflight` with `operation: "rollout"`.
2. Run `k8s.restart_safety_check`.
3. Run `k8s.rollout` action `restart` with `confirm: true`.
4. Run `k8s.rollout` action `status`.
5. Inspect `k8s.events_timeline` for readiness and probe errors.

## Workflow D: Release Scaling

1. Run `k8s.safe_mutation_preflight` with `operation: "scale"`.
2. Run `k8s.scale` to desired replicas.
3. Run `k8s.rollout` action `status`.
4. Run `k8s.resource_usage` to verify headroom remains.

## Deployment Strategy Tie-In

Use `references/STRATEGIES.md` before selecting strategy.

Quick defaults:
- RollingUpdate for most stateless APIs
- Recreate only for singleton or incompatible schema transitions
- Blue-Green for strict zero-downtime cutover requirements
- Canary for high-risk releases needing progressive exposure

## Deployment Spec Example (RollingUpdate)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: payments
spec:
  replicas: 4
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  minReadySeconds: 10
  progressDeadlineSeconds: 600
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
      - name: api
        image: ghcr.io/acme/api:v1.13.0
        ports:
        - containerPort: 8080
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /livez
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        resources:
          requests:
            cpu: "200m"
            memory: "256Mi"
          limits:
            cpu: "1000m"
            memory: "1Gi"
```

## Deployment Spec Example (Recreate)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: worker
  namespace: batch
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: worker
  template:
    metadata:
      labels:
        app: worker
    spec:
      containers:
      - name: worker
        image: ghcr.io/acme/worker:v2.4.1
```

## Helm Values Snippet for Safer Rollouts

```yaml
replicaCount: 4
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 0
podDisruptionBudget:
  enabled: true
  minAvailable: 3
readinessProbe:
  enabled: true
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

## Verification Checklist

- Preflight pass confirmed
- Mutation confirmation explicitly true
- Rollout status healthy
- No new warning events
- No CPU/memory saturation regression
- Service endpoints remain available

## Common Failure Modes

| Symptom | Likely Cause | Primary Check |
|---|---|---|
| rollout never completes | readiness probe mismatch | `k8s.events_timeline` |
| pods crash immediately | bad image or missing config | `k8s.events_timeline` + `k8s.resource_usage` |
| helm upgrade times out | hook/job failure | `helm.status` |
| scale up blocked | quota exhaustion | `k8s.safe_mutation_preflight` |
| restart increases errors | insufficient replica budget | `k8s.restart_safety_check` |

## Escalation Rules

Escalate to `k8s-incident` when:
- rollout status remains non-ready past progress deadline
- warning events indicate systemic failures across services
- node pressure appears during release and affects unrelated workloads

Escalate to `k8s-diagnostics` when:
- post-deploy saturation appears without explicit failures
- autoscaling is unstable after deployment

## Output Contract

Return these sections in every deployment response:
1. preflight findings
2. change executed and exact tool call path
3. rollout verification evidence
4. risk call (continue, hold, or rollback recommendation)

## Cross References

- Strategy details: `skills/claude/k8s-deploy/references/STRATEGIES.md`
- Operational write safety: `skills/claude/k8s-operations/SKILL.md`
- Incident escalation workflow: `skills/claude/k8s-incident/SKILL.md`

End of skill.
