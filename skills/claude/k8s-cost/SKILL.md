# Skill: k8s-cost

Kubernetes cost optimization framework using usage evidence, right-sizing, autoscaling signal quality,
storage hygiene, and node pool efficiency.

This skill focuses on actionable cost reduction without sacrificing reliability.

## Trigger Phrases

Use this skill when the user mentions:
- cluster spend too high
- reduce k8s cost
- overprovisioned workloads
- idle resources
- right sizing
- node utilization low
- too many nodes
- expensive node families
- vpa recommendations
- hpa waste or thrashing
- orphaned pvcs
- immediate cost wins

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.resource_usage`
- `k8s.best_practice`
- `k8s.vpa_debug`
- `k8s.hpa_debug`
- `k8s.storage_debug`
- `k8s.overview`
- `k8s.describe`
- `k8s.list`
- `karpenter.nodepool_debug`
- `karpenter.nodeclass_debug`

## Cost Optimization Principles

1. Start with utilization truth, not YAML intent.
2. Fix biggest waste categories first.
3. Separate immediate savings from architecture changes.
4. Preserve SLOs while reducing slack.
5. Re-run measurements after each change wave.

## End-to-End Cost Workflow

### Phase 1: Baseline and Scope

Use `k8s.overview` for macro shape:
```yaml
namespace: payments
```

Then use `k8s.resource_usage` for live pressure:
```yaml
namespace: payments
includePods: true
includeNodes: true
sortBy: cpu
```

And run memory view:
```yaml
namespace: payments
includePods: true
includeNodes: true
sortBy: memory
```

Capture:
- top pod CPU consumers
- top pod memory consumers
- node utilization spread
- namespaces with low utilization but high reservation

### Phase 2: Workload Right-Sizing

For each top consumer and top overprovisioned candidate:
1. `k8s.describe` workload to capture requests/limits.
2. `k8s.best_practice` to find missing/unsafe resource config.
3. `k8s.vpa_debug` for recommendation baseline.

Example `k8s.best_practice`:
```yaml
kind: Deployment
name: payments-api
namespace: payments
```

Example `k8s.vpa_debug`:
```yaml
namespace: payments
name: payments-api-vpa
```

Right-sizing method:
- If requests >> observed usage and VPA agrees, reduce requests first.
- Keep limits aligned with burst behavior.
- Avoid aggressive cuts for latency-critical services without guardrails.

### Phase 3: Autoscaling Efficiency Review

Use `k8s.hpa_debug` to detect cost amplifiers:
- excessive min replicas
- poor target utilization settings
- thrashing up/down behavior

Example:
```yaml
namespace: payments
name: payments-api
```

Tie HPA output to `k8s.resource_usage`:
- if HPA keeps high replicas with low usage, tune minReplicas/metric targets.
- if HPA lags and causes spikes, right-size requests to improve metric fidelity.

### Phase 4: Storage Waste Review

Run `k8s.storage_debug` for orphaned or problematic claims:
```yaml
namespace: payments
includeEvents: true
```

Then inventory claims:
```yaml
namespace: payments
resources:
  - kind: PersistentVolumeClaim
```

Cost signals:
- old bound PVCs unused by live workloads
- failed mounts leaving retained volumes
- oversized claims for low-use components

### Phase 5: Node Pool Efficiency (Karpenter)

Use `karpenter.nodepool_debug`:
```yaml
name: general-purpose
```

Use `karpenter.nodeclass_debug`:
```yaml
name: default-ec2
```

Assess:
- instance family breadth too wide or too expensive
- consolidation settings disabled
- architecture restrictions preventing cheaper shapes
- mismatch between pod requests and node SKU sizes

## Right-Sizing Methodology (Detailed)

For each workload, compute action class:

### Class A: Heavily overprovisioned

Indicators:
- request/usage ratio > 3x sustained
- low p95 usage relative to requests
- VPA lower bound much below current requests

Action:
- reduce requests in 20-35% steps
- monitor latency/error budget after each step

### Class B: Moderately overprovisioned

Indicators:
- request/usage ratio 1.5x to 3x
- usage has periodic bursts

Action:
- reduce requests conservatively
- preserve higher limits for bursty windows

### Class C: Underprovisioned

Indicators:
- repeated OOM or CPU throttling
- VPA recommends increase

Action:
- increase requests to stabilize scheduling
- avoid false savings that increase incidents

## Unused Resource Detection

Use this checklist:
- Deployments with replicas > 0 but near-zero usage.
- Services without meaningful backing traffic and stale pods.
- PVCs not linked to active StatefulSets or pods.
- Resource limits missing, causing noisy-neighbor inefficiency.

Tools:
- `k8s.resource_usage`
- `k8s.list`
- `k8s.describe`
- `k8s.storage_debug`

## Node Utilization Analysis

Use `k8s.resource_usage` with nodes enabled.

Read patterns:
- Many nodes below 20-30% utilization: likely overcapacity.
- One node saturated and others idle: scheduling/affinity imbalance.
- Memory-fragmented cluster: requests too high or uneven pod sizing.

Then evaluate Karpenter policy:
- `karpenter.nodepool_debug` for constraints and consolidation.
- `karpenter.nodeclass_debug` for infra selection limits.

## Immediate Wins vs Long-Term Strategy

| Category | Immediate Wins (days) | Long-Term Strategy (weeks) | Primary Tools |
|---|---|---|---|
| Requests/limits | Cut obvious 3x+ overprovisioning | Continuous right-sizing loop with review cadence | `k8s.resource_usage`, `k8s.vpa_debug`, `k8s.describe` |
| HPA policy | Tune extreme `minReplicas` values | Metric redesign and scaling objective calibration | `k8s.hpa_debug`, `k8s.resource_usage` |
| Storage | Remove clearly orphaned PVCs | Lifecycle policy for automatic cleanup and retention classes | `k8s.storage_debug`, `k8s.list` |
| Node pools | Enable/adjust consolidation policy | Multi-pool strategy by workload profile | `karpenter.nodepool_debug`, `karpenter.nodeclass_debug` |
| Governance | Apply missing limits quickly | Enforce standards with platform policy controls | `k8s.best_practice`, `k8s.describe` |

## Common Cost Anti-Patterns

1. Every workload copied with same high defaults.
2. HPA min replicas set for peak all day.
3. VPA ignored even when recommendations stable.
4. Large memory requests to avoid occasional OOMs.
5. Stateful workloads never audited after launch.
6. Node pools permit expensive instance families by default.

## Parameter Guidance

### `k8s.resource_usage`

- `includePods: true` for workload ranking.
- `includeNodes: true` for node packing efficiency.
- run twice with `sortBy: cpu` and `sortBy: memory`.

### `k8s.best_practice`

- run per critical workload; track critical findings first.

### `k8s.vpa_debug`

- use named VPA where available to avoid noisy output.

### `k8s.hpa_debug`

- use for every high-replica workload before changing min/max values.

### `k8s.storage_debug`

- include namespace-scoped runs and targeted pod/PVC runs.

### `karpenter.nodepool_debug` and `karpenter.nodeclass_debug`

- map pool constraints to observed workload demand.
- check whether policies prevent cheaper shapes.

## Example YAML: Right-Sized Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: payments
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: api
          image: ghcr.io/acme/payments-api:1.4.3
          resources:
            requests:
              cpu: "250m"
              memory: "384Mi"
            limits:
              cpu: "1000m"
              memory: "768Mi"
```

## Example YAML: Conservative HPA

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: payments-api
  namespace: payments
spec:
  minReplicas: 3
  maxReplicas: 18
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 65
```

## Common Issues Table

| Symptom | Likely Root Cause | Confirm With | Cost Impact | Action |
|---|---|---|---|---|
| High spend, low workload usage | inflated requests across many deployments | `k8s.resource_usage`, `k8s.describe` | high | right-size with VPA guidance |
| Frequent scale-outs at low traffic | HPA sensitivity/min replicas too high | `k8s.hpa_debug` | medium-high | tune HPA policy |
| Large idle PV footprint | orphaned PVCs retained | `k8s.storage_debug`, `k8s.list` | medium | cleanup + lifecycle process |
| Nodes underutilized | node pool shape mismatch | `k8s.resource_usage`, `karpenter.nodepool_debug` | high | tighten pool constraints |
| OOM-driven instability | under-requesting masked by retries | `k8s.vpa_debug`, `k8s.best_practice` | hidden high | increase requests safely |

## Reporting Format

Return:
1. Baseline summary (top CPU, top memory, node utilization).
2. Ranked savings opportunities with confidence level.
3. Immediate actions (safe in current sprint).
4. Long-term actions (platform/process improvements).
5. Risks and validation plan.

Example:
```text
Top opportunity: payments-api requests are 3.4x observed p95 CPU and 2.8x p95 memory.
Estimated effect: fewer nodes after rightsizing and improved bin-packing.
Immediate action: reduce requests by 25% and monitor SLO/error rate for 48h.
Long-term action: monthly VPA review and enforce request/limit policy gate.
```

## Completion Criteria

Cost analysis is complete only when:
- baseline evidence is captured from live metrics,
- at least three prioritized actions are proposed,
- each action includes risk and verification,
- immediate vs long-term strategy is clearly separated.
