# Skill: k8s-networking

Comprehensive Kubernetes networking diagnostics for service reachability, DNS, NetworkPolicy,
Ingress and load balancer health, private endpoint connectivity, and evidence-driven traffic flow analysis.

## Scope

Use this skill when investigating any request that includes connectivity uncertainty between clients,
Ingress, Services, pods, or external dependencies.

This skill is optimized for root cause investigations, not broad architecture redesign.

## Trigger Phrases

Use this skill when the user mentions:
- service unreachable
- ingress 502 or 503
- timeout or connection refused
- dns failure or nxdomain
- endpoint not found
- network policy blocked
- load balancer pending
- private link issues
- cross-namespace traffic denied
- pod can curl ip but not service
- intermittent traffic drops
- health checks failing

## RootCause Tools Allowed

Only use these tool names in this skill:
- `k8s.network_debug`
- `k8s.private_link_debug`
- `k8s.graph`
- `k8s.describe`
- `k8s.list`
- `k8s.events`
- `k8s.exec`

## Investigation Strategy

Follow a strict top-down path:
1. Confirm object existence and shape.
2. Validate service-to-pod wiring.
3. Validate traffic policy constraints.
4. Validate DNS from inside cluster.
5. Validate external path and private connectivity.
6. Build graph evidence and timeline summary.

Do not jump to fixes before evidence from at least two tools agrees.

## Fast Triage Matrix

| Symptom | First Tool | Why |
|---|---|---|
| Service name resolves but requests timeout | `k8s.network_debug` | Fast check for endpoints and policy blocks |
| Ingress returns 404/502/503 | `k8s.graph` | Identifies broken edge between ingress and backend |
| Internal pod cannot resolve service FQDN | `k8s.exec` | DNS proof from same runtime network context |
| Traffic denied after policy deploy | `k8s.network_debug` | Policy decision visibility |
| NLB/ALB health checks failing | `k8s.network_debug` | Service and LB wiring health |
| VPC endpoint/private path failing | `k8s.private_link_debug` | Endpoint-side evidence for private connectivity |

## Primary Workflow

### Step 1: Inventory relevant objects

Use `k8s.list` to map objects before deep diagnostics.

Example parameters:
```yaml
resources:
  - kind: Service
  - kind: Ingress
  - kind: NetworkPolicy
namespace: payments
```

Questions to answer:
- Does the target Service exist in expected namespace?
- Is there an Ingress or Gateway fronting it?
- Are restrictive NetworkPolicies present?

### Step 2: Run service-focused network diagnostics

Use `k8s.network_debug` with the exact namespace and service name.

Example:
```yaml
namespace: payments
service: payments-api
```

Evaluate output in this order:
1. Endpoints populated or empty
2. Service port to targetPort alignment
3. Selector match with running pods
4. NetworkPolicy allow/deny hints
5. LB target health and readiness

### Step 3: Build traffic dependency graph

Use `k8s.graph` to visualize ingress/service/workload/pod chain.

Ingress path example:
```yaml
kind: Ingress
name: payments-public
namespace: payments
```

Service path example:
```yaml
kind: Service
name: payments-api
namespace: payments
```

Look for:
- Missing owner links
- Service selecting zero pods
- Pod readiness false at endpoint targets
- Wrong service port exposed to ingress

### Step 4: Inspect critical objects in detail

Use `k8s.describe` for high-signal resources.

Recommended sequence:
1. Service
2. Ingress
3. NetworkPolicy (suspicious ones)
4. Affected pod

Service focus points:
- `spec.selector`
- `spec.ports[].port`
- `spec.ports[].targetPort`
- endpoint attachment events

Ingress focus points:
- backend service name and port
- class annotation or ingressClassName
- TLS host alignment
- warning events from controller

### Step 5: Validate DNS from workload context

Use `k8s.exec` on a running pod in same namespace when possible.

Preferred command patterns:
```yaml
namespace: payments
pod: payments-worker-7c6f7d8f45-qf9dm
command: ["nslookup", "payments-api.payments.svc.cluster.local"]
```

```yaml
namespace: payments
pod: payments-worker-7c6f7d8f45-qf9dm
command: ["getent", "hosts", "payments-api.payments.svc.cluster.local"]
```

```yaml
namespace: payments
pod: payments-worker-7c6f7d8f45-qf9dm
command: ["sh", "-c", "curl -sv http://payments-api:8080/healthz"]
```

Interpretation:
- DNS fails: cluster DNS or name mismatch issue.
- DNS works but curl fails: service/policy/backend readiness issue.
- Curl works internally but ingress fails: edge/LB/controller issue.

### Step 6: Check event evidence

Use `k8s.events` for warnings in the namespace and on target objects.

Example filters:
```yaml
namespace: payments
involvedObjectKind: Service
involvedObjectName: payments-api
```

```yaml
namespace: payments
involvedObjectKind: Ingress
involvedObjectName: payments-public
```

High-value event patterns:
- failed to create load balancer
- endpoint update failures
- backend not found
- rejected by ingress class

### Step 7: Run private endpoint diagnostics when applicable

Use `k8s.private_link_debug` when path crosses private endpoint boundaries.

Example:
```yaml
namespace: payments
service: payments-api
awsRegion: us-east-1
```

Use this when:
- service is internal-only and accessed via private network
- VPC endpoint service routing suspected
- network path works in-cluster but not from private consumer

## NetworkPolicy Analysis Workflow

1. Use `k8s.list` to enumerate all `NetworkPolicy` in namespace.
2. Use `k8s.describe` for policies targeting affected pods.
3. Compare pod labels versus policy `podSelector`.
4. Validate ingress rules for source namespace/pod selectors.
5. Validate egress rules for DNS and upstream dependencies.
6. Re-run `k8s.network_debug` to confirm observed deny path.

Typical misses:
- no egress rule for DNS (`kube-dns`)
- namespace selector mismatch due missing labels
- port protocol mismatch (TCP vs UDP)
- default deny introduced without compensating allow

## Ingress and Load Balancer Health Workflow

1. `k8s.graph` on Ingress object.
2. `k8s.describe` Ingress for backend and class/controller errors.
3. `k8s.network_debug` on backing service.
4. `k8s.events` for ingress controller warnings.

Evidence cues:
- Ingress path points to non-existent service
- service port mismatch with ingress backend port
- LB health checks target wrong port/path
- endpoints empty due selector drift

## DNS Resolution Testing Patterns

Use `k8s.exec` from two pods when possible:
- caller pod in failing path
- known-good utility pod in same namespace

Command set:
```yaml
command: ["nslookup", "kubernetes.default.svc.cluster.local"]
```

```yaml
command: ["nslookup", "payments-api.payments.svc.cluster.local"]
```

```yaml
command: ["cat", "/etc/resolv.conf"]
```

```yaml
command: ["sh", "-c", "curl -sv http://payments-api.payments.svc.cluster.local:8080/readyz"]
```

## Traffic Flow Graph Analysis Checklist

Use `k8s.graph` output to confirm:
- edge object exists (Ingress/Service)
- backend mapping exists
- workload owner and pods resolved
- pod readiness and endpoint registration
- no dead edge in graph path

If graph is incomplete, inspect missing node with `k8s.describe`.

## Common Error Signatures

| Error Signature | Likely Layer | Root Cause Pattern | Confirm With | Fix Direction |
|---|---|---|---|---|
| `no endpoints available for service` | Service backend | Selector mismatch or pods unready | `k8s.network_debug`, `k8s.describe` | Align labels and readiness gates |
| `connection refused` | Pod/app | Container listening port mismatch | `k8s.describe`, `k8s.exec` | Correct container/service port mapping |
| `i/o timeout` | Network path | Policy deny or LB target unhealthy | `k8s.network_debug`, `k8s.events` | Update policy or LB backend health config |
| `NXDOMAIN` | DNS | Wrong name/namespace or DNS path issue | `k8s.exec` | Use FQDN, validate DNS access |
| `503 upstream unavailable` | Ingress/backend | Ingress route exists but backend empty/unhealthy | `k8s.graph`, `k8s.network_debug` | Restore healthy endpoints |
| `private endpoint timeout` | Private connectivity | Endpoint service/routing/security mismatch | `k8s.private_link_debug` | Correct endpoint configuration |

## Common Investigation Pitfalls

- Debugging ingress first when service itself is broken.
- Testing DNS from local machine instead of pod context.
- Ignoring namespace mismatch in service references.
- Reading only current object state without events.
- Assuming policy allows DNS traffic by default.

## Output Contract

Return results in this structure:
1. Root cause statement with exact object names.
2. Evidence bullets from at least two tools.
3. Immediate mitigation.
4. Permanent fix recommendation.
5. Verification checks to confirm recovery.

Example output template:
```text
Root cause: Service payments/payments-api selects zero pods due label drift.
Evidence: k8s.network_debug shows empty endpoints; k8s.describe Service selector app=payments-api while pods use app=pay-api.
Mitigation: Patch Service selector to app=pay-api.
Permanent fix: Add label conformance check in CI for Service<->Deployment selector parity.
Verification: Re-run k8s.network_debug and k8s.graph; confirm endpoints healthy and ingress path complete.
```

## Escalation Paths

Escalate to adjacent skills when needed:
- `k8s-rollouts` if pod readiness failures originate from bad deploy.
- `k8s-cilium` if CNI-level policy behavior suspected.
- `k8s-certs` if ingress TLS/mutual auth breakage is dominant symptom.

## Done Criteria

This investigation is complete only when:
- failing path is identified at specific layer
- evidence confirms root cause from multiple tools
- at least one concrete mitigation is provided
- verification steps are executable and scoped
