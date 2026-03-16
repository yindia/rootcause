# Skill: k8s-incident

Incident response workflow for Kubernetes outages using RootCause MCP tools only.

Primary goal:
stabilize service fast, preserve evidence, then explain likely causes.

## Core Response Sequence

Follow this exact order unless evidence forces a branch:
1. connectivity check
2. baseline snapshot
3. incident bundle
4. change correlation
5. focused debug
6. event timeline
7. resource pressure scan
8. ranked hypotheses

## Trigger Signals

Use this skill for requests like:
- service down
- outage
- high 5xx
- alert storm
- crashlooping pods
- pending pods
- sudden latency jump
- release appears to have broken traffic

## Severity-Based Triage Table

| Severity | Definition | Initial Objective | First 3 Tools |
|---|---|---|---|
| SEV-1 | customer-critical outage, broad impact | restore service path within minutes | `k8s.ping` → `k8s.overview` → `rootcause.incident_bundle` |
| SEV-2 | major degradation, partial outage | isolate blast radius and prevent escalation | `k8s.ping` → `rootcause.incident_bundle` → `rootcause.change_timeline` |
| SEV-3 | localized failure, workaround exists | identify root component and schedule safe fix | `k8s.overview` → `k8s.events_timeline` → `k8s.diagnose` |
| SEV-4 | low urgency anomaly or warning trend | gather evidence and create prevention actions | `k8s.overview` → `k8s.resource_usage` → `k8s.best_practice` |

## Tool Priority Matrix

| Symptom | Primary Tool | Secondary Tool | Why |
|---|---|---|---|
| API failure | `k8s.ping` | `k8s.context` | verify control plane reachability and context |
| unknown scope | `k8s.overview` | `rootcause.incident_bundle` | broad health then structured evidence |
| crash loops | `k8s.crashloop_debug` | `k8s.logs` | fast root signal from lifecycle and container logs |
| pending pods | `k8s.scheduling_debug` | `k8s.resource_usage` | scheduling blockers and node pressure |
| service unreachable | `k8s.network_debug` | `k8s.graph` | service chain and policy/LB blockers |
| config break | `k8s.config_debug` | `k8s.describe` | missing keys and mount/ref failures |
| storage mount errors | `k8s.storage_debug` | `k8s.events_timeline` | PVC/PV and attach sequence |
| auth/permission errors | `k8s.permission_debug` | `k8s.describe` | RBAC/IRSA binding and runtime failures |

## RootCause Parameter Examples

`k8s.ping`:
```json
{}
```

`k8s.context` current:
```json
{
  "action": "current"
}
```

`k8s.overview` namespace scope:
```json
{
  "namespace": "payments"
}
```

`rootcause.incident_bundle` full chain:
```json
{
  "namespace": "payments",
  "keyword": "checkout errors",
  "includeHelm": true,
  "includeDefaultChain": true,
  "maxSteps": 8,
  "eventLimit": 120,
  "releaseLimit": 15,
  "outputMode": "bundle"
}
```

`rootcause.change_timeline`:
```json
{
  "namespace": "payments",
  "keyword": "checkout",
  "includeHelm": true,
  "includeNormal": false,
  "timelineLimit": 150
}
```

`k8s.crashloop_debug`:
```json
{
  "namespace": "payments",
  "labelSelector": "app=checkout"
}
```

`k8s.scheduling_debug`:
```json
{
  "namespace": "payments",
  "labelSelector": "app=checkout"
}
```

`k8s.network_debug`:
```json
{
  "namespace": "payments",
  "service": "checkout"
}
```

`k8s.storage_debug`:
```json
{
  "namespace": "payments",
  "pod": "checkout-74fd8d6f9b-x8r2c",
  "includeEvents": true
}
```

`k8s.config_debug`:
```json
{
  "namespace": "payments",
  "pod": "checkout-74fd8d6f9b-x8r2c"
}
```

`k8s.permission_debug`:
```json
{
  "namespace": "payments",
  "pod": "checkout-74fd8d6f9b-x8r2c"
}
```

`k8s.events_timeline` warning-focused:
```json
{
  "namespace": "payments",
  "includeNormal": false,
  "limit": 200
}
```

`k8s.resource_usage`:
```json
{
  "namespace": "payments",
  "includePods": true,
  "includeNodes": true,
  "sortBy": "memory",
  "limit": 40
}
```

`k8s.graph` service dependency:
```json
{
  "kind": "Service",
  "name": "checkout",
  "namespace": "payments"
}
```

`k8s.debug_flow` traffic scenario:
```json
{
  "kind": "Service",
  "name": "checkout",
  "namespace": "payments",
  "scenario": "traffic",
  "maxSteps": 8
}
```

`k8s.diagnose` keyword route:
```json
{
  "namespace": "payments",
  "keyword": "image pull backoff",
  "autoFlow": true
}
```

`rootcause.rca_generate`:
```json
{
  "namespace": "payments",
  "keyword": "checkout outage",
  "incidentSummary": "error rate rose to 45% after release"
}
```

`rootcause.remediation_playbook`:
```json
{
  "namespace": "payments",
  "keyword": "checkout outage",
  "maxImmediateActions": 7
}
```

`rootcause.postmortem_export`:
```json
{
  "namespace": "payments",
  "keyword": "checkout outage",
  "incidentSummary": "SEV-1 checkout traffic failure",
  "format": "markdown"
}
```

## Full Incident Workflow

### 1) Connectivity check

Run `k8s.ping` first.

If it fails:
- verify context with `k8s.context`
- stop deeper diagnosis until access is restored

### 2) Baseline snapshot

Run `k8s.overview` for incident namespace.

Capture:
- unhealthy workloads
- pod states
- namespace-level warning signals

### 3) Incident bundle

Run `rootcause.incident_bundle` for aggregated evidence.

Use keyword when known.

Set `includeHelm: true` when release suspicion exists.

### 4) Change correlation

Run `rootcause.change_timeline` to correlate:
- Kubernetes events
- Helm release updates
- ordering between changes and impact onset

### 5) Focused debug by symptom

Choose one primary branch:
- crash branch: `k8s.crashloop_debug`
- scheduling branch: `k8s.scheduling_debug`
- traffic branch: `k8s.network_debug`
- storage branch: `k8s.storage_debug`
- config branch: `k8s.config_debug`
- permission branch: `k8s.permission_debug`

If uncertain, run `k8s.diagnose` with incident keyword.

### 6) Event timeline refinement

Run `k8s.events_timeline` with object filters after branch selection.

Correlate first warning timestamp with deployment or config changes.

### 7) Resource pressure verification

Run `k8s.resource_usage` to detect:
- memory saturation
- CPU throttling risk
- node pressure contamination across namespaces

### 8) Hypothesis synthesis

Generate ranked hypotheses with:
- confidence level
- evidence references
- immediate mitigation
- verification step

Then generate formal artifacts:
- `rootcause.rca_generate`
- `rootcause.remediation_playbook`

## Blast Radius Analysis Technique

Use this five-pass method:

Pass 1: namespace footprint
- run `k8s.overview`
- mark affected workloads

Pass 2: service dependency graph
- run `k8s.graph` for affected Service
- identify upstream/downstream propagation

Pass 3: selector and endpoint integrity
- run `k8s.describe` on Service and Deployment
- confirm selector match and endpoint population

Pass 4: temporal spread
- run `k8s.events_timeline` with warning-only view
- identify whether failures spread or stayed local

Pass 5: resource contagion
- run `k8s.resource_usage` including nodes
- determine if node pressure impacted unrelated apps

## Escalation Paths

Escalate to platform/on-call lead when:
- `k8s.ping` or control-plane access fails
- multi-namespace impact is confirmed
- node pressure or quota collapse affects shared services

Escalate to application team when:
- rollout or image change aligns with first failure signal
- config/schema mismatch is localized to one service
- app-level probes and startup behavior are root driver

Escalate to security/IAM team when:
- `k8s.permission_debug` indicates credential or role break
- token expiration or service account trust policy anomalies appear

Escalate to storage team when:
- `k8s.storage_debug` shows volume attachment or reclaim breakage
- persistent data path is bottlenecking restart and recovery

## Hypothesis Template

Use this format for each hypothesis:

- hypothesis title
- confidence (high/medium/low)
- supporting evidence (tools and key findings)
- contradicting evidence
- immediate mitigation
- verification call

Example verification call:
```json
{
  "namespace": "payments",
  "involvedObjectKind": "Pod",
  "involvedObjectName": "checkout-74fd8d6f9b-x8r2c",
  "includeNormal": false,
  "limit": 50
}
```

## Timebox Guidance

For SEV-1:
- first evidence pass in 10 minutes
- first mitigation candidate in 15 minutes
- confidence-ranked hypotheses in 25 minutes

For SEV-2:
- first evidence pass in 20 minutes
- remediation plan in 40 minutes

## Communication Contract During Incident

Every update should include:
1. current customer impact
2. blast radius estimate
3. current leading hypothesis
4. next concrete check and ETA

## Closure Criteria

Incident response phase closes when:
- impact stabilized or mitigated
- probable causes ranked with evidence
- remediation owner identified
- post-incident artifacts generated

Artifact set:
- `rootcause.rca_generate`
- `rootcause.remediation_playbook`
- `rootcause.postmortem_export`

## Related Skills

- `skills/claude/k8s-core/SKILL.md`
- `skills/claude/k8s-operations/SKILL.md`
- `skills/claude/k8s-diagnostics/SKILL.md`

End of skill.
