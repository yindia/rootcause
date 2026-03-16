# Skill: k8s-autoscaling

Deep autoscaling diagnostics and tuning for three scaling paths:
1) Horizontal Pod Autoscaler (HPA),
2) Vertical Pod Autoscaler (VPA),
3) Karpenter node provisioning.

Use this skill to explain why scaling did or did not happen, then provide a cost-aware remediation path.

## Trigger Phrases

Use this skill when the user mentions:
- hpa not scaling
- replicas stuck
- scale up too slow
- scale down never happens
- vpa recommendation needed
- oomkilled and right sizing
- pods pending no nodes
- karpenter not provisioning
- nodepool constraints
- nodeclass misconfiguration
- spot interruptions
- autoscaling cost too high

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.hpa_debug`
- `k8s.vpa_debug`
- `k8s.resource_usage`
- `k8s.scale` (requires `confirm=true`)
- `k8s.describe`
- `k8s.list`
- `karpenter.status`
- `karpenter.node_provisioning_debug`
- `karpenter.nodepool_debug`
- `karpenter.nodeclass_debug`
- `karpenter.interruption_debug`

## Autoscaling Model

Think in layers:
- Pod replica scaling is controlled by HPA.
- Pod request sizing is controlled by VPA.
- Node supply scaling is controlled by Karpenter.

A healthy system aligns all three layers.

## Decision Tree

| Primary Symptom | Start Here | Next Branch |
|---|---|---|
| CPU high, replicas unchanged | `k8s.hpa_debug` | validate metrics and maxReplicas |
| Recurrent OOMKilled | `k8s.vpa_debug` | compare requests vs recommendations |
| Pods pending with scheduling errors | `karpenter.node_provisioning_debug` | inspect NodePool/NodeClass |
| Node count spikes and costs jump | `k8s.resource_usage` | check over-requesting and HPA sensitivity |
| Frequent node terminations | `karpenter.interruption_debug` | check spot/interruption behavior |

## Workflow A: HPA Diagnostics (CPU/Memory/Custom Metrics)

### Step A1: Enumerate HPAs and targets

Use `k8s.list`:
```yaml
namespace: checkout
resources:
  - kind: HorizontalPodAutoscaler
  - kind: Deployment
```

Confirm target references are valid.

### Step A2: Inspect HPA behavior

Use `k8s.hpa_debug`:
```yaml
namespace: checkout
name: checkout-api
```

Read conditions in order:
1. `AbleToScale`
2. `ScalingActive`
3. `ScalingLimited`

Interpretation examples:
- `ScalingActive=False`: metric pipeline unavailable or invalid target.
- `ScalingLimited=True`: bounded by min/max replicas or stabilization windows.

### Step A3: Compare to live usage

Use `k8s.resource_usage`:
```yaml
namespace: checkout
includePods: true
sortBy: cpu
```

If HPA thinks utilization is low but pods are hot, suspect:
- missing resource requests,
- metrics source lag,
- wrong metric type wiring.

### Step A4: Inspect target deployment spec

Use `k8s.describe` on target workload for:
- resource requests and limits
- probe stability
- rollout state affecting observed metrics

### Step A5: Emergency stabilization

If service is degraded, use manual override:
```yaml
namespace: checkout
kind: Deployment
name: checkout-api
replicas: 12
confirm: true
```

Tool: `k8s.scale`.

Always report this as temporary while root cause is fixed.

## Workflow B: VPA Right-Sizing

### Step B1: List VPA objects and coverage

Use `k8s.list`:
```yaml
namespace: checkout
resources:
  - kind: VerticalPodAutoscaler
  - kind: Deployment
  - kind: StatefulSet
```

Goal: identify workloads without sizing guidance.

### Step B2: Pull VPA recommendations

Use `k8s.vpa_debug`:
```yaml
namespace: checkout
name: checkout-api-vpa
```

Compare:
- `recommendation.containerRecommendations`
- current `requests`

### Step B3: Verify real usage before applying

Use `k8s.resource_usage` (pods sorted by memory and cpu).

Cross-check rule:
- if recommendations are much lower than observed p95 load windows,
  do not apply blindly.

### Step B4: Inspect workload constraints

Use `k8s.describe` to detect:
- min/max VPA policy bounds
- containers excluded from control
- workloads sensitive to restart behavior

### Step B5: Rollout recommendations safely

Apply gradual sizing updates and monitor:
- HPA behavior changes
- restart counts
- node packing impact

## Workflow C: Karpenter Node Provisioning

### Step C1: Control plane health

Use `karpenter.status` first.

If unhealthy, node provisioning analysis downstream is unreliable.

### Step C2: Analyze pending pod constraints

Use `karpenter.node_provisioning_debug`:
```yaml
namespace: checkout
```

Look for blockers:
- incompatible instance requirements
- unsatisfiable zone/architecture constraints
- taints without matching tolerations
- impossible resource requests

### Step C3: Inspect NodePool policy

Use `karpenter.nodepool_debug`:
```yaml
name: general-purpose
```

Check:
- allowed instance families/sizes
- consolidation policy
- capacity limits
- taints and startup taints

### Step C4: Inspect NodeClass wiring

Use `karpenter.nodeclass_debug`:
```yaml
name: default-ec2
```

Check:
- subnet/security group selectors
- AMI family and selectors
- IAM role profile assumptions

### Step C5: Interruption and drift signals

Use `karpenter.interruption_debug` to explain:
- spot reclaim events
- drift replacements
- unexpected churn causing instability

## Common Issues Table

| Symptom | Likely Cause | Confirm With | Fix Direction |
|---|---|---|---|
| HPA never scales above min | metrics inactive | `k8s.hpa_debug` | restore metric source and requests |
| HPA scales late | stabilization/cooldown too conservative | `k8s.hpa_debug` + usage trend | tune behavior windows |
| HPA thrashes up/down | noisy metric or low tolerance | `k8s.hpa_debug` + `k8s.resource_usage` | smooth metric and tune policy |
| OOMKilled despite low HPA utilization | requests too low, memory pressure | `k8s.vpa_debug` + `k8s.describe` | increase requests and limits |
| Pending pods but free cluster capacity appears available | constraints mismatch | `karpenter.node_provisioning_debug` | align tolerations/requirements |
| Karpenter launches expensive nodes | NodePool requirement too broad | `karpenter.nodepool_debug` | constrain instance families |
| Unexpected node churn | interruption/drift | `karpenter.interruption_debug` | adjust disruption/consolidation strategy |

## Cost-Optimized Scaling Patterns

### Pattern 1: HPA + conservative requests

- Use VPA recommendations to remove inflated requests.
- Keep HPA on utilization metrics tied to realistic requests.
- Result: better bin-packing and fewer nodes.

### Pattern 2: Separate burst and steady pools

- Use dedicated NodePools for bursty workloads.
- Keep steady baseline on stable instance families.
- Result: lower volatility and predictable baseline spend.

### Pattern 3: Guardrails against runaway scale

- Set sensible `maxReplicas` per critical service.
- Use alerting on sustained max replica saturation.
- Pair with Karpenter capacity caps.

### Pattern 4: Use manual scale only as incident brake

- `k8s.scale` with `confirm=true` is for mitigation, not long-term policy.
- After incident, restore autoscaling governance.

## Parameter Guidance

### `k8s.hpa_debug`

- `namespace`: required for scoped analysis.
- `name`: optional but recommended to isolate one HPA.

### `k8s.vpa_debug`

- `namespace`: required.
- `name`: optional for targeted workload inspection.

### `k8s.resource_usage`

- `includePods`: true for workload-level signal.
- `includeNodes`: true for capacity-level signal.
- `sortBy`: `cpu` or `memory` depending on symptom.

### `k8s.scale`

- Always include `confirm: true`.
- Set exact `kind`, `name`, `namespace`, and `replicas`.

### `karpenter.node_provisioning_debug`

- `namespace`: recommended for narrowing pending pods.
- `pods`: use for specific critical pods.

## Example YAML Snippets

### HPA example

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: checkout-api
  namespace: checkout
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: checkout-api
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 65
```

### VPA example

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: checkout-api-vpa
  namespace: checkout
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: checkout-api
  updatePolicy:
    updateMode: "Off"
```

### Karpenter NodePool example

```yaml
apiVersion: karpenter.sh/v1
kind: NodePool
metadata:
  name: general-purpose
spec:
  template:
    spec:
      requirements:
        - key: kubernetes.io/arch
          operator: In
          values: ["amd64"]
```

## Output Contract

Return:
1. Primary scaling failure mode (HPA, VPA, or Karpenter).
2. Evidence from at least two tools.
3. Immediate mitigation (if incident active).
4. Durable scaling fix.
5. Cost impact expectation (increase, decrease, neutral).

## Completion Criteria

Autoscaling diagnosis is complete only if:
- scaling blocker is tied to a concrete object/condition
- recommended change references the right control plane (HPA/VPA/Karpenter)
- mitigation and long-term fix are separated
- cost implication is explicitly called out
