# Kubernetes Common Errors (RootCause MCP)

Comprehensive troubleshooting reference for high-frequency Kubernetes failures.
All tool names use RootCause MCP `k8s.*` format.

---

## How To Use This Reference

1. Match your observed message to the closest section.
2. Run the listed primary tool first.
3. Validate with follow-up tools before deciding remediation.
4. Use `k8s.events_timeline` to confirm the first causal failure.

---

## CrashLoopBackOff

### Meaning

Container starts, fails, and is restarted repeatedly with exponential backoff.

### Frequent Causes

- Application startup exception.
- Missing environment variable, secret, or config key.
- Wrong command/entrypoint arguments.
- External dependency unavailable at startup.
- Probe configuration too strict causing kill loops.

### RootCause Tools

- Primary: `k8s.crashloop_debug`
- Follow-up: `k8s.logs`, `k8s.describe`, `k8s.events_timeline`
- Pivot tools: `k8s.config_debug`, `k8s.permission_debug`, `k8s.network_debug`

### Resolution Pattern

1. Run `k8s.crashloop_debug` and capture top finding.
2. Pull `k8s.logs` for stack trace and first fatal line.
3. Use `k8s.describe` for restart reasons and probe status.
4. If message references missing config, run `k8s.config_debug`.
5. If auth is denied, run `k8s.permission_debug`.

---

## ImagePullBackOff / ErrImagePull

### Meaning

Kubelet cannot pull container image from registry.

### Frequent Causes

- Wrong image name or tag.
- Registry auth failure.
- Missing image pull secret.
- Role/trust issue for cloud registry access.
- Registry/network transient failure.

### RootCause Tools

- Primary: `k8s.crashloop_debug`
- Follow-up: `k8s.describe`, `k8s.events_timeline`, `k8s.events`
- Pivot tools: `k8s.permission_debug`

### Resolution Pattern

1. Run `k8s.crashloop_debug` for pull error categorization.
2. Use `k8s.describe` to read exact pull reason.
3. Use `k8s.events_timeline` to confirm repeated pull backoff.
4. If auth denied, run `k8s.permission_debug` for SA/IRSA path.
5. Correct image/tag/credentials, then verify new pull succeeds.

---

## OOMKilled (Exit 137)

### Meaning

Container exceeded memory limit and was terminated.

### Frequent Causes

- Limit too low for workload profile.
- Memory leak or burst not covered by limits.
- Sidecar overhead not included in sizing.
- Traffic spike without sufficient scale.

### RootCause Tools

- Primary: `k8s.logs`
- Follow-up: `k8s.describe`, `k8s.resource_usage`
- Pivot tools: `k8s.vpa_debug`, `k8s.hpa_debug`

### Resolution Pattern

1. Read `k8s.logs` for OOM markers around restart time.
2. Confirm `Last State` reason with `k8s.describe`.
3. Check runtime/node pressure using `k8s.resource_usage`.
4. Pull sizing guidance from `k8s.vpa_debug`.
5. Validate scaling behavior via `k8s.hpa_debug`.

---

## Exit Code 1

### Meaning

Generic application error; process exited unsuccessfully.

### Frequent Causes

- Unhandled exception on startup.
- Bad arguments or missing required env.
- Dependency connection failure.

### RootCause Tools

- Primary: `k8s.logs`
- Follow-up: `k8s.crashloop_debug`, `k8s.describe`
- Pivot tools: `k8s.config_debug`, `k8s.network_debug`

### Resolution Pattern

1. Inspect exact fatal line using `k8s.logs`.
2. Run `k8s.crashloop_debug` for restart context.
3. Use `k8s.describe` for probe and event evidence.
4. Route to config/network debuggers based on log text.

---

## Exit Code 127

### Meaning

Command not found in container.

### Frequent Causes

- Invalid command/entrypoint path.
- Base image missing binary.
- Script file not mounted or not executable.

### RootCause Tools

- Primary: `k8s.logs`
- Follow-up: `k8s.describe`, `k8s.crashloop_debug`
- Pivot tools: `k8s.exec`

### Resolution Pattern

1. Confirm `not found` error in `k8s.logs`.
2. Verify command/args in `k8s.describe` workload spec.
3. Use `k8s.exec` in a healthy sibling pod/image when needed to verify paths.

---

## Exit Code 128+N (Signal Termination)

### Meaning

Process terminated by signal (`128 + signal number`).

### Frequent Causes

- `143` (`SIGTERM`) during rolling update/shutdown.
- `137` (`SIGKILL`) due to OOM or forced kill.
- Abrupt node/process termination.

### RootCause Tools

- Primary: `k8s.describe`
- Follow-up: `k8s.logs`, `k8s.events_timeline`
- Pivot tools: `k8s.resource_usage`

### Resolution Pattern

1. Inspect termination reason in `k8s.describe`.
2. Correlate termination timing with `k8s.events_timeline`.
3. If memory suspected, validate with `k8s.resource_usage`.

---

## Scheduling Errors (`0/N nodes available`)

### Meaning

Scheduler cannot place pod on any node.

### Frequent Causes

- Resource requests exceed available allocatable capacity.
- Node selector/affinity mismatch.
- Missing toleration for taints.
- Quota/policy constraints.

### RootCause Tools

- Primary: `k8s.scheduling_debug`
- Follow-up: `k8s.events_timeline`, `k8s.describe`, `k8s.resource_usage`
- Pivot tools: `k8s.debug_flow` (`pending`)

### Resolution Pattern

1. Run `k8s.scheduling_debug` to identify dominant blocker.
2. Validate chronology and retries with `k8s.events_timeline`.
3. Confirm actual pressure with `k8s.resource_usage`.
4. Use `k8s.debug_flow` for multi-factor pending incidents.

---

## Storage Errors (`FailedMount`, `FailedAttachVolume`)

### Meaning

Pod cannot bind, attach, or mount required volume.

### Frequent Causes

- PVC not bound.
- PV class/capacity/access mode mismatch.
- CSI node/plugin issues.
- Mount permission/path issues inside container.

### RootCause Tools

- Primary: `k8s.storage_debug`
- Follow-up: `k8s.describe`, `k8s.events_timeline`, `k8s.logs`
- Pivot tools: `k8s.exec`

### Resolution Pattern

1. Run `k8s.storage_debug` for storage lifecycle status.
2. Check pod/PVC details with `k8s.describe`.
3. Use `k8s.events_timeline` to locate first bind/attach failure.
4. If mount succeeds but app fails, inspect with `k8s.logs` and `k8s.exec`.

---

## Network Errors (`Connection refused`, `i/o timeout`, `no route`)

### Meaning

Traffic path from client to backend is broken or blocked.

### Frequent Causes

- Service selector mismatch (no endpoints).
- Wrong target port or named port mismatch.
- NetworkPolicy denies source/destination.
- Backend pod not ready.

### RootCause Tools

- Primary: `k8s.network_debug`
- Follow-up: `k8s.graph`, `k8s.describe`, `k8s.logs`
- Pivot tools: `k8s.debug_flow` (`traffic` or `networkpolicy`)

### Resolution Pattern

1. Run `k8s.network_debug` on failing service.
2. Run `k8s.graph` to inspect ingress/service/pod dependency chain.
3. Use `k8s.describe` for selector/endpoints details.
4. Confirm backend readiness with `k8s.logs`.

---

## RBAC Errors (`forbidden`, `cannot list/get/watch`)

### Meaning

ServiceAccount lacks required permissions for attempted action.

### Frequent Causes

- Role/ClusterRole missing required verbs.
- Binding exists in wrong namespace.
- Workload using unexpected ServiceAccount.

### RootCause Tools

- Primary: `k8s.permission_debug`
- Follow-up: `k8s.describe`, `k8s.events_timeline`, `k8s.logs`
- Pivot tools: `k8s.get`

### Resolution Pattern

1. Run `k8s.permission_debug` for SA->RBAC chain analysis.
2. Confirm denied verb/resource from `k8s.logs`.
3. Correlate admission/auth failures with `k8s.events_timeline`.

---

## IRSA / Cloud IAM Auth Errors

### Meaning

Pod cannot assume or use intended cloud role credentials.

### Frequent Causes

- Missing/wrong ServiceAccount role annotation.
- IAM trust policy mismatch.
- Audience/token configuration mismatch.

### RootCause Tools

- Primary: `k8s.permission_debug`
- Follow-up: `k8s.logs`, `k8s.describe`

### Resolution Pattern

1. Run `k8s.permission_debug` and inspect role binding evidence.
2. Validate auth errors from SDK logs via `k8s.logs`.
3. Confirm ServiceAccount attachment in `k8s.describe`.

---

## Admission Webhook Errors

### Meaning

Object creation/update blocked by validating or mutating webhook.

### Frequent Causes

- Policy rejection by governance controls.
- Webhook service timeout/unreachable.
- TLS or CA bundle mismatch.

### RootCause Tools

- Primary: `k8s.events`
- Follow-up: `k8s.events_timeline`, `k8s.describe`, `k8s.network_debug`
- Pivot tools: `k8s.diagnose`

### Resolution Pattern

1. Run `k8s.events` scoped to failing resource.
2. Confirm sequence and retries with `k8s.events_timeline`.
3. Use `k8s.describe` for full admission failure message.
4. If timeout suspected, inspect webhook path via `k8s.network_debug`.

---

## Probe Failures (`Readiness probe failed`, `Liveness probe failed`)

### Meaning

Kubelet marks container unhealthy due to failed probe checks.

### Frequent Causes

- App startup slower than probe thresholds.
- Probe path/port incorrect.
- Dependency unavailable, causing probe endpoint failure.
- CPU starvation delaying probe response.

### RootCause Tools

- Primary: `k8s.logs`
- Follow-up: `k8s.describe`, `k8s.network_debug`, `k8s.resource_usage`
- Pivot tools: `k8s.crashloop_debug`

### Resolution Pattern

1. Check probe failure logs with `k8s.logs`.
2. Read probe config and events in `k8s.describe`.
3. Validate endpoint/network path with `k8s.network_debug`.
4. Check saturation and throttling signals via `k8s.resource_usage`.

---

## Deployment Not Progressing (`ProgressDeadlineExceeded`)

### Meaning

Deployment rollout did not complete in expected progress window.

### Frequent Causes

- New pods stuck Pending.
- New pods crash on startup.
- Pods running but never Ready due to probe/network/config issues.

### RootCause Tools

- Primary: `k8s.diagnose`
- Follow-up: `k8s.debug_flow`, `k8s.scheduling_debug`, `k8s.crashloop_debug`, `k8s.network_debug`

### Resolution Pattern

1. Start with `k8s.diagnose` using keyword `deployment stuck`.
2. Run `k8s.debug_flow` based on dominant symptom.
3. Follow state-specific tools to isolate blocker.
4. Confirm with `k8s.events_timeline` and `k8s.logs`.

---

## Config Errors (`secret not found`, `configmap not found`, missing key)

### Meaning

Container cannot resolve required config object/key at runtime.

### Frequent Causes

- Typo in object/key name.
- Object created in wrong namespace.
- Non-optional reference points to absent key.

### RootCause Tools

- Primary: `k8s.config_debug`
- Follow-up: `k8s.describe`, `k8s.logs`, `k8s.get`

### Resolution Pattern

1. Run `k8s.config_debug` for key/object validation.
2. Use `k8s.describe` to confirm exact failing reference.
3. Confirm runtime impact via `k8s.logs`.

---

## Quick Lookup Table

| Error / Symptom | Primary Tool | Key Follow-up | Typical Fix Direction |
|---|---|---|---|
| CrashLoopBackOff | `k8s.crashloop_debug` | `k8s.logs`, `k8s.describe` | Fix startup/config/auth issue |
| ImagePullBackOff | `k8s.crashloop_debug` | `k8s.describe`, `k8s.events_timeline` | Correct image/tag/auth |
| OOMKilled / 137 | `k8s.logs` | `k8s.resource_usage`, `k8s.vpa_debug` | Increase memory / optimize app |
| Exit 1 | `k8s.logs` | `k8s.crashloop_debug` | Fix app runtime error |
| Exit 127 | `k8s.logs` | `k8s.describe` | Correct command/image binary |
| Exit 128+N | `k8s.describe` | `k8s.events_timeline` | Track signal source and stabilize |
| Pending (`0/N nodes`) | `k8s.scheduling_debug` | `k8s.resource_usage` | Fix constraints/capacity |
| FailedMount / Attach | `k8s.storage_debug` | `k8s.describe`, `k8s.events_timeline` | Fix PVC/PV/CSI path |
| Service unreachable | `k8s.network_debug` | `k8s.graph`, `k8s.logs` | Fix selector/policy/port/readiness |
| RBAC forbidden | `k8s.permission_debug` | `k8s.logs`, `k8s.describe` | Add required role bindings |
| IRSA auth failure | `k8s.permission_debug` | `k8s.logs` | Correct SA annotation/trust |
| Admission denied | `k8s.events` | `k8s.describe`, `k8s.events_timeline` | Fix policy or webhook health |
| Probe failed | `k8s.logs` | `k8s.describe`, `k8s.network_debug` | Adjust probes / fix dependency |
| Deployment stuck | `k8s.diagnose` | `k8s.debug_flow` | Resolve underlying pending/crash/network issue |
| Config missing key/object | `k8s.config_debug` | `k8s.describe`, `k8s.logs` | Correct object/key/namespace |

---

## Cross-Cutting Best Practices

- For layered incidents, always run `k8s.diagnose` first.
- Use `k8s.debug_flow` to keep investigation path deterministic.
- Validate root cause with at least one event and one log artifact.
- Avoid fixing secondary symptoms before first causal blocker is confirmed.
