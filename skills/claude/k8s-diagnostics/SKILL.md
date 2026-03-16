# Skill: k8s-diagnostics

Cluster and workload diagnostics playbook using RootCause MCP tool names.

Focus areas:
- cluster health assessment
- resource usage and pressure
- event chronology
- HPA and VPA behavior
- best-practice interpretation
- capacity planning

## Objective

Produce evidence-backed health posture and action priorities.

This skill is mostly read-first.
If remediation requires mutation, hand off to `k8s-operations`.

## Primary Tool Set

- `k8s.context`
- `k8s.ping`
- `k8s.overview`
- `k8s.resource_usage`
- `k8s.events`
- `k8s.events_timeline`
- `k8s.hpa_debug`
- `k8s.vpa_debug`
- `k8s.best_practice`
- `k8s.list`
- `k8s.describe`
- `k8s.graph`

## Trigger Conditions

Activate for requests like:
- cluster health check
- resource hotspots
- scaling seems wrong
- noisy warnings across namespace
- capacity planning ahead of release
- post-incident verification

## Baseline Diagnostic Sequence

1. verify active context with `k8s.context`
2. verify control plane with `k8s.ping`
3. collect baseline with `k8s.overview`
4. detect pressure with `k8s.resource_usage`
5. correlate warnings with `k8s.events_timeline`
6. inspect autoscaling using `k8s.hpa_debug` and `k8s.vpa_debug`
7. score workload configuration with `k8s.best_practice`
8. produce prioritized recommendations

## Parameter Examples

`k8s.context` current:
```json
{
  "action": "current"
}
```

`k8s.context` list:
```json
{
  "action": "list"
}
```

`k8s.ping`:
```json
{}
```

`k8s.overview` namespace:
```json
{
  "namespace": "payments"
}
```

`k8s.resource_usage` cpu hotspot view:
```json
{
  "namespace": "payments",
  "includePods": true,
  "includeNodes": true,
  "sortBy": "cpu",
  "limit": 40
}
```

`k8s.resource_usage` memory hotspot view:
```json
{
  "namespace": "payments",
  "includePods": true,
  "includeNodes": true,
  "sortBy": "memory",
  "limit": 40
}
```

`k8s.events` namespace warnings and normals:
```json
{
  "namespace": "payments"
}
```

`k8s.events_timeline` warning-only:
```json
{
  "namespace": "payments",
  "includeNormal": false,
  "limit": 150
}
```

`k8s.events_timeline` object-focused:
```json
{
  "namespace": "payments",
  "involvedObjectKind": "Deployment",
  "involvedObjectName": "api",
  "includeNormal": false,
  "limit": 80
}
```

`k8s.hpa_debug` specific HPA:
```json
{
  "namespace": "payments",
  "name": "api"
}
```

`k8s.hpa_debug` all HPA in namespace:
```json
{
  "namespace": "payments"
}
```

`k8s.vpa_debug` specific VPA:
```json
{
  "namespace": "payments",
  "name": "api"
}
```

`k8s.best_practice` deployment:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.list` deployments by team label:
```json
{
  "namespace": "payments",
  "labelSelector": "team=checkout",
  "resources": [
    { "kind": "Deployment" }
  ]
}
```

`k8s.describe` deployment:
```json
{
  "kind": "Deployment",
  "name": "api",
  "namespace": "payments"
}
```

`k8s.graph` service dependency map:
```json
{
  "kind": "Service",
  "name": "api",
  "namespace": "payments"
}
```

## Cluster Health Assessment Model

Assess five dimensions:
1. control-plane reachability
2. workload readiness
3. resource pressure
4. event instability
5. scaling governance

### Dimension 1: control-plane reachability

Tools:
- `k8s.context`
- `k8s.ping`

Outcome:
- reachable and trusted context
- or blocked due to auth/network

### Dimension 2: workload readiness

Tools:
- `k8s.overview`
- `k8s.list`
- `k8s.describe`

Key indicators:
- non-ready pods
- deployment available replica lag
- repeated probe failures

### Dimension 3: resource pressure

Tools:
- `k8s.resource_usage`

Key indicators:
- top memory consumers
- top CPU consumers
- node saturation trend

### Dimension 4: event instability

Tools:
- `k8s.events`
- `k8s.events_timeline`

Key indicators:
- warning density increase
- repeating scheduler failures
- image pull or mount failures

### Dimension 5: scaling governance

Tools:
- `k8s.hpa_debug`
- `k8s.vpa_debug`
- `k8s.best_practice`

Key indicators:
- HPA condition mismatch
- VPA recommendation drift from requests/limits
- missing resource constraints and probes

## Resource Usage Analysis Patterns

### Pattern A: namespace hotspot sweep

1. run cpu sorted usage
2. run memory sorted usage
3. compare top 10 overlap
4. map top pods to deployments
5. inspect corresponding HPA/VPA state

### Pattern B: node pressure contamination

1. include nodes in usage
2. identify nodes near saturation
3. list pods placed on hot nodes
4. determine whether impact crosses teams

Node contamination query:
```json
{
  "fieldSelector": "spec.nodeName=ip-10-0-32-17.ec2.internal",
  "resources": [
    { "kind": "Pod" }
  ]
}
```

### Pattern C: pre-release stress baseline

1. collect usage snapshot before deploy window
2. capture warning timeline for last hour
3. archive best-practice score for release workloads
4. compare post-release values for regression

## Event Analysis and Timeline Techniques

Technique 1: warning-first timeline
- use `includeNormal: false`
- identify first anomalous event

Technique 2: object-focused sequence
- filter by object kind + name
- establish event causality for one workload

Technique 3: namespace spread detection
- run timeline for namespace with high limit
- detect if warnings fan out over time

Technique 4: rate burst heuristic
- compare warning count in 10-minute windows
- if spike aligns with release marker, raise confidence on change hypothesis

## HPA Analysis Guide

Use `k8s.hpa_debug` to inspect:
- current replicas
- desired replicas
- scaling conditions
- target metric status

Interpretation patterns:
- desired > current with no scale-up can imply resource constraints or stale metrics
- current pinned at max implies underprovisioned baseline or incorrect target
- frequent oscillation implies unstable metric target or noisy demand

Follow-up checks:
- `k8s.resource_usage`
- `k8s.events_timeline`
- `k8s.describe` target workload

## VPA Analysis Guide

Use `k8s.vpa_debug` to inspect:
- recommendation confidence
- target workload mapping
- update policy behavior

Interpretation patterns:
- no recommendations can indicate insufficient observation window
- recommendations far above limits indicate chronic throttling or memory under-requesting
- recommendations far below requests indicate reclaim opportunity

Follow-up checks:
- `k8s.resource_usage`
- `k8s.best_practice`

## Best Practice Scoring Interpretation

Use `k8s.best_practice` score as risk signal, not as sole decision maker.

Typical high-impact findings:
- missing resource limits
- missing readiness/liveness probes
- single replica critical workload
- broad security misconfigurations

Interpret by severity bands:
- critical findings: action before next release
- major findings: schedule in near-term sprint
- minor findings: track in hygiene backlog

## Capacity Planning Workflow

### Step 1: collect current baseline

Tools:
- `k8s.overview`
- `k8s.resource_usage`

### Step 2: identify critical workloads

Tools:
- `k8s.list` for production labels
- `k8s.hpa_debug` for autoscaled services

### Step 3: evaluate headroom and risks

Signals:
- node utilization near limits
- high replica demand near HPA max
- repeated resource pressure warnings

### Step 4: quality controls

Tools:
- `k8s.best_practice`
- `k8s.describe`

### Step 5: recommendation set

Produce:
- immediate mitigation actions
- near-term right-sizing tasks
- long-term scaling architecture actions

## Pre-Deployment Checklist

Before major release windows:
- verify context and connectivity
- run namespace overview
- capture cpu/memory usage baseline
- check warning event trend
- inspect HPA for target workloads
- inspect VPA recommendations if used
- run best-practice checks for release workloads

Suggested calls:

```json
{
  "namespace": "payments"
}
```

```json
{
  "namespace": "payments",
  "includePods": true,
  "sortBy": "cpu",
  "limit": 50
}
```

```json
{
  "namespace": "payments",
  "includeNormal": false,
  "limit": 120
}
```

## Post-Incident Checklist

After stabilization:
- rerun overview for incident namespace
- verify warning trend decays
- verify usage returns to expected envelope
- check autoscaler conditions normalize
- run best-practice checks on impacted workloads
- map dependency graph for residual blast radius

Suggested calls:

```json
{
  "namespace": "payments",
  "name": "api"
}
```

```json
{
  "namespace": "payments",
  "kind": "Deployment",
  "name": "api"
}
```

```json
{
  "kind": "Service",
  "name": "api",
  "namespace": "payments"
}
```

## Troubleshooting Matrix

| Symptom | Likely Cause | Primary Tool | Secondary Tool |
|---|---|---|---|
| empty resource metrics | metrics backend unavailable | `k8s.resource_usage` | `k8s.events` |
| HPA not reacting | metric pipeline or target config issue | `k8s.hpa_debug` | `k8s.describe` |
| VPA no output | insufficient runtime data | `k8s.vpa_debug` | `k8s.resource_usage` |
| warning flood | cascading dependency or resource contention | `k8s.events_timeline` | `k8s.graph` |
| healthy overview but SLO bad | external dependency or network path issue | `k8s.graph` | `k8s.events_timeline` |

## Output Contract

Diagnostic responses must include:
1. context and connectivity status
2. baseline health summary
3. hotspot table (cpu/memory, pods/nodes)
4. warning timeline interpretation
5. autoscaling state summary
6. best-practice risk findings
7. prioritized actions (immediate, near-term, long-term)

## Related Skills

- `skills/claude/k8s-core/SKILL.md`
- `skills/claude/k8s-operations/SKILL.md`
- `skills/claude/k8s-incident/SKILL.md`

End of skill.
