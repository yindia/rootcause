# Skill: k8s-cilium

Comprehensive Cilium diagnostics for control-plane detection, endpoint health,
network policy behavior, and connectivity failures.

Includes practical CiliumNetworkPolicy examples and Hubble-centric investigation concepts.

## Trigger Phrases

Use this skill when the user mentions:
- cilium issue
- cni problem
- cilium endpoint not ready
- policy denied traffic
- dropped traffic
- hubble flow gap
- pod to pod connectivity failed
- service reachable from some pods only
- egress policy blocked

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.cilium_detect`
- `k8s.diagnose_cilium`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.network_debug`

## Cilium Workflow Overview

Canonical sequence:
1. detect Cilium footprint,
2. diagnose Cilium health,
3. inspect affected endpoints and policies,
4. correlate with service-level network evidence.

## Step 1: Detect Cilium Installation

Use `k8s.cilium_detect` first.

Outcome handling:
- detected: continue with Cilium-specific diagnosis.
- not detected: stop Cilium branch and pivot to generic network diagnostics.

## Step 2: Run Cluster/Namespace Diagnosis

Use `k8s.diagnose_cilium` for aggregated state.

Example:
```yaml
namespace: payments
limit: 100
```

Read for:
- endpoint health summary,
- policy status warnings,
- node-level Cilium issues,
- recurring warning events.

## Step 3: Inventory Cilium Resources

Use `k8s.list` for policy and endpoint visibility.

Example:
```yaml
namespace: payments
resources:
  - kind: CiliumNetworkPolicy
  - kind: CiliumEndpoint
  - kind: CiliumNode
```

## Step 4: Inspect Target Resources

Use `k8s.describe` in this priority order:
1. failing CiliumEndpoint,
2. governing CiliumNetworkPolicy,
3. affected service/pod context.

Watch for:
- identity allocation errors,
- endpoint policy enforcement modes,
- selector mismatch in policies,
- stale labels after rollout.

## Step 5: Correlate with Service Path

Use `k8s.network_debug` on affected service to validate:
- endpoint selection,
- potential policy denials,
- load balancer path effects.

Example:
```yaml
namespace: payments
service: payments-api
```

## Network Policy Management Guidance

### Checklist for policy correctness

1. Does policy select intended pods?
2. Are ingress and egress both addressed as needed?
3. Are DNS egress paths allowed where required?
4. Are namespace labels used in selectors accurate?
5. Are ports/protocols aligned with app behavior?

### Typical policy failure patterns

- policy targets no pods due label drift
- default deny introduced without allow exceptions
- missing UDP 53 egress for DNS
- namespace selector assumes missing label
- policy order assumptions that do not apply

## Endpoint Health Investigation

When endpoint is not ready:
1. `k8s.list` CiliumEndpoints in namespace,
2. `k8s.describe` specific endpoint,
3. `k8s.events` for pod/namespace warnings,
4. rerun `k8s.diagnose_cilium` to check if systemic.

Signals to capture:
- endpoint state transitions,
- identity status,
- policy revision mismatches,
- CNI attach failures.

## Hubble Flow Debugging Concepts

Even when direct Hubble CLI is not used, think in flow terms:
- source identity
- destination identity
- L4/L7 policy verdict
- dropped vs forwarded transitions

Use available `k8s.diagnose_cilium` and `k8s.events` outputs as proxy signals for these flow decisions.

Practical approach:
1. identify source pod and destination service,
2. map policies selecting source and destination,
3. verify events for denied traffic indicators,
4. confirm service graph integrity with `k8s.network_debug`.

## YAML Examples

### Example CiliumNetworkPolicy allowing namespace-scoped traffic

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-web-to-api
  namespace: payments
spec:
  endpointSelector:
    matchLabels:
      app: api
  ingress:
    - fromEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: payments
            app: web
      toPorts:
        - ports:
            - port: "8080"
              protocol: TCP
```

### Example policy allowing DNS egress

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: allow-dns-egress
  namespace: payments
spec:
  endpointSelector:
    matchLabels:
      app: api
  egress:
    - toEndpoints:
        - matchLabels:
            k8s:io.kubernetes.pod.namespace: kube-system
            k8s:k8s-app: kube-dns
      toPorts:
        - ports:
            - port: "53"
              protocol: UDP
            - port: "53"
              protocol: TCP
```

## Troubleshooting Table

| Symptom | Likely Cause | Confirm With | Fix Direction |
|---|---|---|---|
| Pod-to-pod traffic denied | selector mismatch or missing allow rule | `k8s.describe` CNP + `k8s.network_debug` | update policy selectors/rules |
| Endpoint not ready | identity/CNI failure | `k8s.describe` endpoint + `k8s.events` | resolve endpoint initialization issue |
| Service reachable internally, not externally | service/LB path mismatch plus policy effects | `k8s.network_debug` + policy inspection | repair service path and policy boundaries |
| Intermittent drops | policy churn or node-level instability | `k8s.diagnose_cilium` + events | stabilize policy rollout and node health |
| DNS lookups fail after policy change | egress DNS not allowed | policy review + service behavior | add DNS egress allowance |

## Parameter Guidance

### `k8s.diagnose_cilium`

- run namespace-scoped first for focused incidents.
- run cluster-wide for systemic or multi-namespace outages.

### `k8s.list`

- include both Cilium custom resources and core workloads for mapping.

### `k8s.events`

- include namespace and object filters to avoid noise.

### `k8s.network_debug`

- use exact service name and namespace for precise traffic path evidence.

## Output Contract

Always provide:
1. detection result,
2. endpoint health summary,
3. policy objects likely involved,
4. concrete root-cause statement,
5. corrective actions and validation plan.

Example:
```text
Root cause: payments/allow-web-to-api policy stopped matching source pods after label rename app=frontend.
Evidence: k8s.describe CiliumNetworkPolicy shows fromEndpoints app=web; k8s.network_debug reports deny path for web->api.
Fix: update policy selector to app=frontend and validate endpoint policy revision convergence.
Verify: rerun k8s.network_debug and confirm successful service requests.
```

## Completion Criteria

Cilium diagnosis is complete when:
- the affected policy/endpoint pair is identified,
- evidence confirms causal path,
- remediation is actionable,
- verification checks are defined.
