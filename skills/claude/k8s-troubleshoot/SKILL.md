# k8s-troubleshoot
Comprehensive Kubernetes troubleshooting skill for pod, service, node, storage, config, and permission failures.
This skill prioritizes RootCause MCP tools in `k8s.*` format and avoids non-RootCause aliases.

## Scope

- Diagnose unknown failures quickly with `k8s.diagnose`.
- Run scenario-accurate graph analysis with `k8s.debug_flow`.
- Pivot into specialized debuggers for crash loops, scheduling, networking, storage, config, and RBAC.
- Produce evidence-backed findings with logs, events, object status, and resource usage.

## Hard Rules

- Always start from symptom -> state -> tool mapping.
- Use RootCause MCP tool names only (`k8s.logs`, `k8s.describe`, `k8s.crashloop_debug`, etc.).
- Never start with random object browsing when a state-specific debugger exists.
- Always collect both runtime evidence (`k8s.logs`) and control-plane evidence (`k8s.events_timeline`).
- When multiple symptoms appear, run `k8s.diagnose` first, then chain `k8s.debug_flow` by scenario.

## Trigger Phrases

- Pod is failing
- Pod keeps restarting
- CrashLoopBackOff
- ImagePullBackOff
- Deployment rollout stuck
- Pods pending
- Service unreachable
- 503 from ingress
- PVC not binding
- Mount failed
- Secret or config not found
- Forbidden RBAC error
- Autoscaling not working
- Node pressure or evictions

## Tool Quick Map

| Need | Primary Tool | Follow-up Tools |
|---|---|---|
| Unknown or mixed symptom | `k8s.diagnose` | `k8s.debug_flow`, `k8s.events_timeline` |
| Crash/restart/image pulls | `k8s.crashloop_debug` | `k8s.logs`, `k8s.describe` |
| Pending/scheduling | `k8s.scheduling_debug` | `k8s.resource_usage`, `k8s.events_timeline` |
| Service connectivity | `k8s.network_debug` | `k8s.graph`, `k8s.debug_flow` |
| Storage failures | `k8s.storage_debug` | `k8s.describe`, `k8s.events_timeline` |
| ConfigMap/Secret issues | `k8s.config_debug` | `k8s.describe`, `k8s.logs` |
| RBAC/IRSA permission | `k8s.permission_debug` | `k8s.describe`, `k8s.events_timeline` |
| HPA behavior | `k8s.hpa_debug` | `k8s.resource_usage`, `k8s.events_timeline` |
| VPA recommendations | `k8s.vpa_debug` | `k8s.resource_usage`, `k8s.describe` |
| Timeline and causality | `k8s.events_timeline` | `k8s.events`, `k8s.describe` |

## Decision Tree: Pod State -> First Tool -> Follow-up

| Pod State / Symptom | First Tool | First Follow-up | Deep Follow-up |
|---|---|---|---|
| `CrashLoopBackOff` | `k8s.crashloop_debug` | `k8s.logs` | `k8s.config_debug` or `k8s.permission_debug` |
| `ImagePullBackOff` | `k8s.crashloop_debug` | `k8s.describe` | `k8s.events_timeline` |
| `ErrImagePull` | `k8s.crashloop_debug` | `k8s.events` | `k8s.permission_debug` (IRSA/ECR paths) |
| `Pending` | `k8s.scheduling_debug` | `k8s.events_timeline` | `k8s.resource_usage` |
| `ContainerCreating` too long | `k8s.describe` | `k8s.events_timeline` | `k8s.storage_debug` or `k8s.network_debug` |
| `OOMKilled` / exit `137` | `k8s.logs` | `k8s.resource_usage` | `k8s.vpa_debug` |
| Frequent restarts with `Running` | `k8s.logs` | `k8s.describe` | `k8s.crashloop_debug` |
| `CreateContainerConfigError` | `k8s.config_debug` | `k8s.describe` | `k8s.events_timeline` |
| `CreateContainerError` | `k8s.logs` | `k8s.describe` | `k8s.config_debug` |
| `Init:CrashLoopBackOff` | `k8s.crashloop_debug` | `k8s.logs` | `k8s.storage_debug` |
| `Terminating` stuck | `k8s.describe` | `k8s.events_timeline` | `k8s.logs` |
| Evicted / node pressure | `k8s.scheduling_debug` | `k8s.resource_usage` | `k8s.events_timeline` |
| Probe failures | `k8s.logs` | `k8s.describe` | `k8s.network_debug` |
| Service not reachable | `k8s.network_debug` | `k8s.graph` | `k8s.debug_flow` (`traffic`) |
| Access denied / forbidden | `k8s.permission_debug` | `k8s.describe` | `k8s.events_timeline` |

## Fast Workflow (Default)

1. Get namespace context and symptom from user report.
2. Run `k8s.diagnose` with a keyword that matches the symptom.
3. Run `k8s.debug_flow` using the closest scenario (`traffic`, `pending`, `crashloop`, `autoscaling`, `networkpolicy`, `mesh`, `permission`).
4. Collect evidence: `k8s.logs`, `k8s.describe`, `k8s.events_timeline`.
5. Pivot to specialized debug tool based on first hard error found.
6. Confirm impact scope with `k8s.list` and `k8s.graph` when traffic is involved.
7. Produce fix recommendation with explicit evidence chain.

## Scenario Workflow: CrashLoopBackOff / ImagePullBackOff

### Goal

Find whether failure is application startup, image pull auth/tag, missing config, or permission.

### Tool Chain

1. `k8s.crashloop_debug` for high-signal diagnosis.
2. `k8s.logs` for exact stack trace and exit reason.
3. `k8s.describe` for container state transitions and event reasons.
4. `k8s.events_timeline` for pull retries and timing correlation.
5. If config mention appears, run `k8s.config_debug`.
6. If image auth/IAM appears, run `k8s.permission_debug`.

### Common Findings

- Image tag missing or wrong registry repo.
- Private image pull denied due to auth/role mismatch.
- Startup command fails (`exec format error`, missing binary, bad args).
- Config key/secret key missing at container startup.

### Output Contract

- Include first failing event line.
- Include one representative log line proving root cause.
- Include the exact resource and namespace.

## Scenario Workflow: Pending / Scheduling Failures

### Goal

Determine if scheduler is blocked by resources, constraints, or policy.

### Tool Chain

1. `k8s.scheduling_debug` for quota, taint/toleration, affinity, and node-fit blockers.
2. `k8s.events_timeline` for scheduler decision chronology.
3. `k8s.resource_usage` for cluster pressure and allocatable context.
4. `k8s.describe` on pod for full scheduling messages.
5. Optional `k8s.list` for similar workloads sharing same constraints.

### Common Findings

- Insufficient CPU/memory or ephemeral storage.
- Node selectors or affinity rules match zero nodes.
- Missing toleration for tainted nodes.
- ResourceQuota / LimitRange prevents admission.

## Scenario Workflow: OOM and Resource Saturation

### Goal

Separate memory leaks/usage spikes from limit misconfiguration.

### Tool Chain

1. `k8s.logs` for OOM markers and shutdown behavior.
2. `k8s.resource_usage` for current pod and node pressure.
3. `k8s.describe` for `Last State` and termination reason.
4. `k8s.vpa_debug` for right-sizing guidance.
5. `k8s.hpa_debug` if scaling behavior is also abnormal.

### Common Findings

- Limit lower than startup/runtime working set.
- Sudden traffic increase without autoscaling reaction.
- Sidecar memory overhead not accounted for.

## Scenario Workflow: Service Not Accessible / Network Failures

### Goal

Locate break in ingress -> service -> endpoints -> pod chain.

### Tool Chain

1. `k8s.network_debug` for service and policy level checks.
2. `k8s.graph` to visualize dependency path.
3. `k8s.debug_flow` with `traffic` or `networkpolicy` scenario.
4. `k8s.describe` on service/workload for endpoint and selector facts.
5. `k8s.logs` on backend pod for probe and request traces.

### Common Findings

- Service selector mismatch yields zero endpoints.
- Port name/targetPort mismatch.
- NetworkPolicy denies namespace or pod selector path.
- Health probes failing leads to no ready endpoints.

## Scenario Workflow: Storage / Volume Problems

### Goal

Find where bind, attach, or mount lifecycle is blocked.

### Tool Chain

1. `k8s.storage_debug` for PVC/PV/VolumeAttachment diagnostics.
2. `k8s.describe` on pod and PVC for detailed failure reason.
3. `k8s.events_timeline` for bind/attach/mount ordering.
4. `k8s.logs` if app reports filesystem or permission errors.

### Common Findings

- PVC pending due to missing storage class or capacity mismatch.
- PV access mode mismatch (`ReadWriteOnce` vs multi-node usage).
- CSI attach timeout or node plugin issue.
- Mount succeeds but filesystem permissions break app startup.

## Scenario Workflow: ConfigMap / Secret / Env Wiring

### Goal

Prove missing key/object vs wrong reference path.

### Tool Chain

1. `k8s.config_debug` for object/key checks and pod references.
2. `k8s.describe` for event-level reference errors.
3. `k8s.logs` for startup failures caused by missing env/files.
4. `k8s.get` on workload object to confirm field paths if needed.

### Common Findings

- Secret key typo in `envFrom` or `valueFrom`.
- ConfigMap exists in wrong namespace.
- Optional=false on missing key causes hard startup fail.

## Scenario Workflow: RBAC / IAM / IRSA Permission

### Goal

Identify denied action and missing permission binding.

### Tool Chain

1. `k8s.permission_debug` for ServiceAccount + RBAC + IAM role path.
2. `k8s.describe` for admission and auth-related warnings.
3. `k8s.events_timeline` for denied requests and retries.
4. `k8s.logs` for SDK-level auth error strings.

### Common Findings

- ServiceAccount not bound to required Role/ClusterRole.
- Wrong namespace role binding target.
- IRSA annotation mismatch to IAM role.
- Token audience or trust policy mismatch on cloud calls.

## Multi-Symptom Investigation Pattern

Use this when symptoms overlap (for example Pending + CrashLoop + 503):

1. Run `k8s.diagnose` with highest-impact keyword first.
2. Run `k8s.debug_flow` with matching primary scenario.
3. Pull `k8s.events_timeline` to detect ordering (what failed first).
4. Use `k8s.graph` if any traffic symptom exists.
5. Split into tracks:
   - Runtime track: `k8s.logs`, `k8s.crashloop_debug`
   - Scheduling track: `k8s.scheduling_debug`
   - Network track: `k8s.network_debug`
   - Config/Auth track: `k8s.config_debug`, `k8s.permission_debug`
6. Converge on earliest causal blocker, not latest secondary symptom.

## Deep Debug Chains (Step-by-Step)

### Chain A: Restart + Probe Flap + 503

1. `k8s.diagnose` with keyword `service unreachable`.
2. `k8s.debug_flow` scenario `traffic`.
3. `k8s.network_debug` on target service.
4. `k8s.graph` for path validation.
5. `k8s.logs` on backend pod for readiness failures.
6. `k8s.describe` for probe event cadence.
7. If restarts seen, run `k8s.crashloop_debug`.

### Chain B: Pending then CrashLoop after schedule

1. `k8s.diagnose` with keyword `pending`.
2. `k8s.scheduling_debug` to clear placement blockers.
3. `k8s.events_timeline` to confirm scheduling success timestamp.
4. Once running, if restarts begin: `k8s.crashloop_debug`.
5. `k8s.logs` for startup failure.
6. If config mention appears: `k8s.config_debug`.

### Chain C: Deploy succeeds but app cannot access dependency

1. `k8s.diagnose` with keyword `permission denied`.
2. `k8s.permission_debug` for SA/RBAC/IRSA verification.
3. `k8s.logs` for exact denied action and API/service target.
4. `k8s.events` for webhook/admission denials.
5. If network side suspected too: `k8s.network_debug`.

## Error Message Guide (High Signal)

| Error Text Pattern | Likely Cause | First Tool | Confirm With |
|---|---|---|---|
| `Back-off restarting failed container` | App startup failure loop | `k8s.crashloop_debug` | `k8s.logs` |
| `ErrImagePull` / `ImagePullBackOff` | Image tag/auth/repo issue | `k8s.crashloop_debug` | `k8s.describe` |
| `OOMKilled` / `exit code 137` | Memory pressure or low limit | `k8s.logs` | `k8s.resource_usage` |
| `0/.. nodes are available` | Scheduling constraints/resource shortage | `k8s.scheduling_debug` | `k8s.events_timeline` |
| `FailedMount` / `FailedAttachVolume` | PVC/PV/CSI path issue | `k8s.storage_debug` | `k8s.describe` |
| `secret "..." not found` | Missing secret or wrong namespace | `k8s.config_debug` | `k8s.describe` |
| `configmap "..." not found` | Missing config object or key | `k8s.config_debug` | `k8s.describe` |
| `forbidden` / `cannot list resource` | RBAC permission missing | `k8s.permission_debug` | `k8s.logs` |
| `Readiness probe failed` | App not ready or dependency unavailable | `k8s.logs` | `k8s.network_debug` |
| `dial tcp` / `i/o timeout` | Service DNS/network path issue | `k8s.network_debug` | `k8s.graph` |

## Evidence Checklist Before Declaring Root Cause

- At least one control-plane evidence item from `k8s.events_timeline` or `k8s.describe`.
- At least one runtime evidence item from `k8s.logs`.
- A direct mapping from symptom to failing component.
- A validated next action tied to specific resource.

## References

- [Decision Tree Reference](references/DECISION-TREE.md)
- [Common Errors Reference](references/COMMON-ERRORS.md)
