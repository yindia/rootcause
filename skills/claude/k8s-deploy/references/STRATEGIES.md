# Deployment Strategies Reference

This reference compares rollout strategies for Kubernetes workloads using RootCause MCP tooling.

Only RootCause tool names are used in this document.

## Purpose

Use this file to decide between:
- RollingUpdate
- Recreate
- Blue-Green
- Canary

Then execute safely with preflight and verification.

## Mandatory Safety Baseline

Always run these checks before any strategy execution:
1. `k8s.safe_mutation_preflight`
2. `k8s.best_practice`
3. `k8s.restart_safety_check` when restarts are involved

All write tools must use `confirm: true`.

## Comparison Table

| Strategy | Downtime Risk | Rollback Speed | Complexity | Infra Cost | Best For |
|---|---|---|---|---|---|
| RollingUpdate | low | medium | low | low | most stateless services |
| Recreate | high | medium | low | low | singleton apps or hard incompatibility windows |
| Blue-Green | very low | very fast | medium | high | strict availability and instant rollback requirements |
| Canary | very low if controlled | fast to medium | high | medium | risky releases requiring progressive validation |

## Decision Flow Diagram

```text
Start
  |
  |-- Need zero downtime and immediate rollback?
  |       |-- Yes --> Blue-Green
  |       |-- No
  |
  |-- Is release high risk or behavior uncertain?
  |       |-- Yes --> Canary
  |       |-- No
  |
  |-- Can old and new versions run together safely?
  |       |-- No --> Recreate
  |       |-- Yes --> RollingUpdate
  |
End
```

## RootCause Tool Mapping by Strategy

| Strategy | Preflight | Deploy Action | Verify | Escalation |
|---|---|---|---|---|
| RollingUpdate | `k8s.safe_mutation_preflight` | `k8s.apply` or `helm.upgrade` | `k8s.rollout` + `k8s.events_timeline` + `k8s.resource_usage` | `rootcause.incident_bundle` |
| Recreate | `k8s.safe_mutation_preflight` | `k8s.apply` or `helm.upgrade` with recreate config | `k8s.events_timeline` + service checks | `rootcause.change_timeline` |
| Blue-Green | `k8s.safe_mutation_preflight` | `k8s.create` or `k8s.apply` for green stack + service switch via `k8s.patch` | `k8s.get` + `k8s.describe` + `k8s.events_timeline` | `rootcause.incident_bundle` |
| Canary | `k8s.safe_mutation_preflight` | staged `k8s.apply`/`k8s.patch`/`helm.upgrade` | `k8s.resource_usage` + `k8s.events_timeline` + `k8s.graph` | `rootcause.rca_generate` |

## RollingUpdate

### How it works

Kubernetes replaces pods gradually based on `maxSurge` and `maxUnavailable`.

Traffic remains available when readiness probes are correct.

### Minimal YAML

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
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8080
```

### RootCause call sequence

`k8s.safe_mutation_preflight`:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n..."
}
```

`k8s.apply`:
```json
{
  "confirm": true,
  "namespace": "payments",
  "fieldManager": "rootcause-release",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: api\n..."
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

### Pros

- low operational complexity
- no duplicate full environment needed
- native Deployment behavior

### Cons

- rollback is not instantaneous
- mixed-version window can expose compatibility issues
- requires robust readiness/liveness design

### Use when

- stateless API
- backward-compatible schema changes
- normal release cadence

## Recreate

### How it works

Old ReplicaSet is terminated before new pods are started.

This creates a downtime window.

### Minimal YAML

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ledger-sync
  namespace: finance
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: ledger-sync
  template:
    metadata:
      labels:
        app: ledger-sync
    spec:
      containers:
      - name: sync
        image: ghcr.io/acme/ledger-sync:v3.2.0
```

### RootCause call sequence

`k8s.safe_mutation_preflight`:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "ledger-sync",
  "namespace": "finance",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: ledger-sync\n..."
}
```

`k8s.apply`:
```json
{
  "confirm": true,
  "namespace": "finance",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: ledger-sync\n..."
}
```

`k8s.events_timeline`:
```json
{
  "namespace": "finance",
  "involvedObjectKind": "Deployment",
  "involvedObjectName": "ledger-sync",
  "includeNormal": false,
  "limit": 80
}
```

### Pros

- simple behavior
- avoids dual-version concurrency conflicts

### Cons

- expected downtime
- elevated incident risk for user-facing paths

### Use when

- strict singleton workload
- non-compatible startup/migration behavior
- maintenance window accepted

## Blue-Green

### How it works

Run two full versions in parallel:
- blue (current)
- green (new)

Switch traffic at the Service or ingress layer.

Rollback is immediate by reversing selector/routing.

### Reference YAML (two Deployments + selector switch)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout-blue
  namespace: shop
spec:
  replicas: 4
  selector:
    matchLabels:
      app: checkout
      version: blue
  template:
    metadata:
      labels:
        app: checkout
        version: blue
    spec:
      containers:
      - name: checkout
        image: ghcr.io/acme/checkout:v1.20.4
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout-green
  namespace: shop
spec:
  replicas: 4
  selector:
    matchLabels:
      app: checkout
      version: green
  template:
    metadata:
      labels:
        app: checkout
        version: green
    spec:
      containers:
      - name: checkout
        image: ghcr.io/acme/checkout:v1.21.0
---
apiVersion: v1
kind: Service
metadata:
  name: checkout
  namespace: shop
spec:
  selector:
    app: checkout
    version: blue
  ports:
  - port: 80
    targetPort: 8080
```

### RootCause call sequence

Preflight green apply:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "checkout-green",
  "namespace": "shop",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: checkout-green\n..."
}
```

Create or apply green:
```json
{
  "confirm": true,
  "namespace": "shop",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: checkout-green\n..."
}
```

Patch Service selector cutover:
```json
{
  "confirm": true,
  "kind": "Service",
  "name": "checkout",
  "namespace": "shop",
  "patchType": "merge",
  "patch": "{\"spec\":{\"selector\":{\"app\":\"checkout\",\"version\":\"green\"}}}"
}
```

Verify cutover:
```json
{
  "kind": "Service",
  "name": "checkout",
  "namespace": "shop"
}
```

### Pros

- near-zero downtime
- fastest rollback by selector revert
- clear isolation between old and new versions

### Cons

- doubles runtime footprint during overlap
- requires explicit traffic switching control
- can complicate data migration coordination

### Use when

- hard SLO requirements
- business-critical user paths
- rollback speed is top priority

## Canary

### How it works

Release new version to a small subset first.

Observe error rates, latency, and saturation.

Increase traffic gradually if healthy.

### Example YAML pattern (weighted split via labels/services)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: recommendation-stable
  namespace: shop
spec:
  replicas: 9
  selector:
    matchLabels:
      app: recommendation
      track: stable
  template:
    metadata:
      labels:
        app: recommendation
        track: stable
    spec:
      containers:
      - name: app
        image: ghcr.io/acme/reco:v5.2.1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: recommendation-canary
  namespace: shop
spec:
  replicas: 1
  selector:
    matchLabels:
      app: recommendation
      track: canary
  template:
    metadata:
      labels:
        app: recommendation
        track: canary
    spec:
      containers:
      - name: app
        image: ghcr.io/acme/reco:v5.3.0
```

### RootCause call sequence

Preflight canary apply:
```json
{
  "operation": "apply",
  "kind": "Deployment",
  "name": "recommendation-canary",
  "namespace": "shop",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: recommendation-canary\n..."
}
```

Apply canary:
```json
{
  "confirm": true,
  "namespace": "shop",
  "manifest": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: recommendation-canary\n..."
}
```

Observe canary pressure:
```json
{
  "namespace": "shop",
  "includePods": true,
  "sortBy": "memory",
  "limit": 50
}
```

Check failure sequence:
```json
{
  "namespace": "shop",
  "involvedObjectKind": "Pod",
  "includeNormal": false,
  "limit": 120
}
```

Analyze traffic dependency graph:
```json
{
  "kind": "Service",
  "name": "recommendation",
  "namespace": "shop"
}
```

### Pros

- controlled blast radius
- strong confidence building before full rollout
- good for high-risk code paths

### Cons

- requires careful monitoring discipline
- can be operationally complex
- route-split implementation varies by stack

### Use when

- new behavior uncertainty is high
- previous regressions justify staged rollout
- strong observability is available

## Helm Strategy Notes

You can use all four strategies with Helm charts by shaping values.

Typical mappings:
- RollingUpdate: default Deployment strategy values
- Recreate: set `strategy.type=Recreate`
- Blue-Green: chart supports dual releases or versioned selectors
- Canary: chart supports parallel canary workload values

Pre-upgrade call:
```json
{
  "release": "checkout",
  "namespace": "shop",
  "chart": "checkout",
  "repoURL": "https://charts.example.internal",
  "version": "3.7.0",
  "valuesFiles": ["values/prod.yaml"]
}
```

Upgrade call:
```json
{
  "confirm": true,
  "release": "checkout",
  "namespace": "shop",
  "chart": "checkout",
  "repoURL": "https://charts.example.internal",
  "version": "3.7.0",
  "valuesFiles": ["values/prod.yaml"],
  "atomic": true,
  "wait": true,
  "timeoutSeconds": 900
}
```

Status call:
```json
{
  "release": "checkout",
  "namespace": "shop"
}
```

## Guardrails for Strategy Selection

Choose RollingUpdate when uncertain and service is stateless.

Choose Recreate only with explicit downtime acceptance.

Choose Blue-Green when rollback speed is mission critical.

Choose Canary when risk is high and metrics can support staged gates.

## Incident-Aware Escalation

If strategy rollout fails, run:
1. `rootcause.incident_bundle`
2. `rootcause.change_timeline`
3. `rootcause.rca_generate`

Example incident bundle call:
```json
{
  "namespace": "shop",
  "keyword": "checkout rollout",
  "includeHelm": true,
  "includeDefaultChain": true,
  "maxSteps": 6,
  "outputMode": "bundle"
}
```

Example timeline call:
```json
{
  "namespace": "shop",
  "keyword": "checkout",
  "includeHelm": true,
  "includeNormal": false,
  "timelineLimit": 120
}
```

Example RCA call:
```json
{
  "namespace": "shop",
  "keyword": "checkout",
  "incidentSummary": "error spike after green cutover"
}
```

## Quick Strategy Checklist

Before deploy:
- confirm objective and availability target
- run preflight and quality checks
- pick strategy using decision flow

During deploy:
- execute one mutation step at a time
- verify with rollout/events/usage between steps

After deploy:
- check warning timeline stability
- confirm SLO indicators externally
- record strategy outcome for future tuning

End of reference.
